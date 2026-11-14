package engine

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/notes"
)

// lastBusinessDay is the last Monday of the world: no deadline falls after it.
const lastBusinessDay = 7

// factsOf reads the dealer's note of a request once and remembers what it means.
func (e *Engine) factsOf(req *api.PriceRequest) notes.Facts {
	if f, ok := e.facts[req.ID]; ok {
		return f
	}
	f := notes.Parse("", req.ArrivedAt)
	if req.Note != nil {
		f = notes.Parse(req.Note.Text, req.ArrivedAt)
	}
	e.facts[req.ID] = f
	return f
}

// narrowsHours reports whether a note gives a site shorter hours than the request states.
// Drivers plan by the stated hours, so such a car needs windows of its own.
func narrowsHours(stated *api.OpeningHours, opens, closes int) bool {
	if stated == nil {
		return false
	}
	return (opens != notes.None && opens > stated.Opens) || (closes != notes.None && closes < stated.Closes)
}

// realHours is a site's stated hours with what the note says of them: the hours the
// site really keeps on weekdays. Saturday hours are never touched by a note.
func realHours(stated *api.OpeningHours, opens, closes int) *api.OpeningHours {
	if stated == nil || !narrowsHours(stated, opens, closes) {
		return stated
	}
	real := *stated
	if opens != notes.None && opens > real.Opens {
		real.Opens = opens
	}
	if closes != notes.None && closes < real.Closes {
		real.Closes = closes
	}
	return &real
}

// realLeg is the first car of a request as its note says it really is: real site hours.
func (e *Engine) realLeg(req *api.PriceRequest) *api.Leg {
	leg := &req.Legs[0]
	if e.P.Notes != 1 || req.Note == nil {
		return leg
	}
	f := e.factsOf(req)
	real := *leg
	real.Pickup.Hours = realHours(leg.Pickup.Hours, f.PickupOpens, f.PickupCloses)
	real.Delivery.Hours = realHours(leg.Delivery.Hours, f.DeliveryOpens, f.DeliveryCloses)
	return &real
}

// narrowed reports whether the note of a request shortens the hours of one of its sites.
func (e *Engine) narrowed(req *api.PriceRequest) bool {
	if e.P.Notes != 1 || req.Note == nil {
		return false
	}
	f := e.factsOf(req)
	leg := &req.Legs[0]
	return narrowsHours(leg.Pickup.Hours, f.PickupOpens, f.PickupCloses) || narrowsHours(leg.Delivery.Hours, f.DeliveryOpens, f.DeliveryCloses)
}

// noteRefusal names why a request's note makes it not worth quoting, or "" when it does not.
// Shorter hours are handled by day windows (NoteHours 1) for a single car between two
// dealer sites; other cases are declined.
func (e *Engine) noteRefusal(req *api.PriceRequest) string {
	if e.P.Notes != 1 || req.Note == nil {
		return ""
	}
	f := e.factsOf(req)
	leg := &req.Legs[0]
	switch {
	case f.SummerTyres:
		return "summer tyres"
	case !e.narrowed(req):
		return ""
	case e.P.NoteHours != 1 || len(req.Legs) > 1 || leg.Pickup.PrivateAddress || leg.Delivery.PrivateAddress:
		return "shorter hours"
	}
	return ""
}

// shiftBusinessDays moves a deadline by whole days, landing on a business day and not
// after the last Monday. A car ready a day or more later moves its deadline the same way.
func shiftBusinessDays(deadline, days int) int {
	if days <= 0 {
		return deadline
	}
	day := cal.DayOf(deadline) + days
	for !cal.IsBusinessDay(day) {
		day++
	}
	return cal.At(min(day, lastBusinessDay), cal.MinuteOfDay(deadline))
}

// effective is what really holds for the first car of a request on one delivery option,
// after its note: when it can first be collected, and by when it must arrive.
func (e *Engine) effective(req *api.PriceRequest, opt api.DeliveryOption) (ready, deadline int) {
	leg := &req.Legs[0]
	ready, deadline = max(req.RespondBy, leg.Pickup.Window.From), opt.Deadline
	if e.P.Notes != 1 || req.Note == nil {
		return ready, deadline
	}
	f := e.factsOf(req)
	if f.NotBefore != notes.None && f.NotBefore > leg.Pickup.Window.From {
		deadline = shiftBusinessDays(deadline, cal.DayOf(f.NotBefore)-cal.DayOf(leg.Pickup.Window.From))
		ready = max(ready, f.NotBefore)
	}
	if f.Deadline != notes.None {
		deadline = min(deadline, f.Deadline)
	}
	return ready, deadline
}

// applyFacts records on a won order what its note demands of the gig.
func (e *Engine) applyFacts(ob *orderBook) {
	if e.P.Notes != 1 || ob.req == nil || ob.req.Note == nil || len(ob.legs) == 0 {
		return
	}
	f := e.factsOf(ob.req)
	lb := &ob.legs[0]
	if f.NotBefore != notes.None {
		lb.notBefore = max(lb.notBefore, f.NotBefore)
	}
	lb.needManual = lb.needManual || f.Manual
	lb.needPlates = lb.needPlates || f.Plates
	if f.Deadline != notes.None {
		ob.noteDeadline = f.Deadline
	}
	// The books carry the hours the sites really keep; drivers still plan by the stated
	// ones, so a car with shorter real hours is posted with windows of its own, day by day.
	leg := &ob.order.Legs[0]
	if narrowsHours(leg.Pickup.Hours, f.PickupOpens, f.PickupCloses) || narrowsHours(leg.Delivery.Hours, f.DeliveryOpens, f.DeliveryCloses) {
		ob.narrowed = true
		leg.Pickup.Hours = realHours(leg.Pickup.Hours, f.PickupOpens, f.PickupCloses)
		leg.Delivery.Hours = realHours(leg.Delivery.Hours, f.DeliveryOpens, f.DeliveryCloses)
	}
}

// dayRoomMin is the least room (minutes) a day must leave for the pickup to be worth posting.
const dayRoomMin = 45

// dayWindows finds the first day on which a car with shorter real hours can be collected
// and delivered inside them, and returns the windows of that day for both stops. late
// tells that the day is after the deadline. Both sites are dealer sites.
func (e *Engine) dayWindows(ob *orderBook, buffer int) (pickup, delivery api.TimeWindow, late, ok bool) {
	leg, lb := &ob.order.Legs[0], &ob.legs[0]
	ready := max(e.now, leg.Pickup.Window.From, lb.notBefore)
	drive := e.driveMin(leg)
	limit := ob.deadlineOf() - buffer
	for day := cal.DayOf(ready); cal.At(day, 0) < e.endsAt; day++ {
		pickOpens, pickCloses, pickOK := cal.HoursOn(leg.Pickup.Hours, day)
		delOpens, delCloses, delOK := cal.HoursOn(leg.Delivery.Hours, day)
		if !pickOK || !delOK {
			continue
		}
		arriveBy := cal.At(day, delCloses) - buffer
		late = cal.At(day, delOpens) > limit
		if !late {
			arriveBy = min(arriveBy, limit)
		}
		from := max(ready, cal.At(day, pickOpens), cal.At(day, delOpens)-drive)
		to := min(cal.At(day, pickCloses), arriveBy-drive)
		if to-from < dayRoomMin {
			continue
		}
		pickup = api.TimeWindow{From: from, To: to}
		delivery = api.TimeWindow{From: max(leg.Delivery.Window.From, cal.At(day, delOpens)), To: arriveBy}
		return pickup, delivery, late, true
	}
	return pickup, delivery, false, false
}

// deadlineOf is the minute by which the order must arrive: the order's deadline, or the
// note's when that is earlier. The world's lastPickupAt does not know the note.
func (ob *orderBook) deadlineOf() int {
	if ob.noteDeadline > 0 && ob.noteDeadline < ob.order.Deadline {
		return ob.noteDeadline
	}
	return ob.order.Deadline
}
