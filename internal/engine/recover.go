package engine

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// Failed-visit reasons.
const (
	reasonNotReady = "not_ready"
	reasonClosed   = "closed"
	reasonLicence  = "licence"
	reasonNoPlates = "no_plates"
)

// Outcomes of a cancelled order's gigs.
const (
	outcomeWithdrawn = "withdrawn"
	outcomeReleased  = "released"
)

func (e *Engine) onWon(o *api.Order, at int) {
	if o == nil {
		return
	}
	if _, known := e.orders[o.ID]; known {
		return
	}
	req := e.reqs[o.ID]
	if req == nil {
		// Never seen the request (a restart): assume a typical fee from the price.
		req = &api.PriceRequest{ID: o.ID, DealerID: o.DealerID, ReferenceFeeCents: o.PriceCents * 3 / 4}
		e.reqs[o.ID] = req
	}
	ob := &orderBook{order: *o, req: req, legs: make([]legBook, len(o.Legs)), wonAt: at}
	for i, d := range o.DeliveredAt {
		if d != nil && i < len(ob.legs) {
			ob.legs[i].delivered = true
		}
	}
	e.orders[o.ID] = ob
	e.stats.Won++
	if day := cal.DayOf(at); day != e.winsDay {
		e.winsDay, e.winsToday = day, 0
	}
	e.winsToday++
	e.onQuoteAnswer(o.ID, true)
	e.applyFacts(ob)
	e.dispatch(ob)
}

// onChanged takes the dealer's change of plan (or a deadline moved by snow).
func (e *Engine) onChanged(o *api.Order, msg *api.Note) {
	if o == nil {
		return
	}
	ob := e.orders[o.ID]
	if ob == nil {
		return
	}
	ob.order = *o
	e.applyFacts(ob)
	e.stats.Changed++
	if msg != nil {
		e.logf("CHANGED %s: %s", o.ID, msg.Text)
	}
	// An open gig is posted again, so its windows, buffer and fee fit the changed order.
	for _, g := range e.gigsOf(ob) {
		e.repost(g)
	}
	e.dispatch(ob)
}

// gigsOf lists the live gigs that carry a car of the order.
func (e *Engine) gigsOf(ob *orderBook) []*gigBook {
	var out []*gigBook
	for i := range ob.legs {
		g := ob.legs[i].gig
		if g == nil || !g.live() {
			continue
		}
		seen := false
		for _, have := range out {
			seen = seen || have == g
		}
		if !seen {
			out = append(out, g)
		}
	}
	return out
}

func (e *Engine) onCancelled(ev *api.Event) {
	ob := e.orders[ev.OrderID]
	if ob == nil {
		return
	}
	ob.cancelled = true
	e.stats.Cancelled++
	for i := range ob.legs {
		ob.legs[i].gig = nil
	}
	for _, c := range ev.Gigs {
		g := e.gigs[c.GigID]
		if g == nil {
			continue
		}
		others := g.legs[:0:0]
		for _, ref := range g.legs {
			if ref.orderID != ev.OrderID {
				others = append(others, ref)
			}
		}
		g.legs = others
		if c.Outcome != outcomeWithdrawn && c.Outcome != outcomeReleased {
			continue // shortened or continues: the gig goes on with its other cars
		}
		g.status = gigWithdrawn
		e.freeLegs(g)
	}
}

// freeLegs takes the cars of a dead gig off it and posts them again.
func (e *Engine) freeLegs(g *gigBook) {
	var owners []*orderBook
	for _, ref := range g.legs {
		ob := e.orders[ref.orderID]
		if ob == nil {
			continue
		}
		if lb := &ob.legs[ref.leg]; lb.gig == g {
			lb.gig = nil
		}
		known := false
		for _, have := range owners {
			known = known || have == ob
		}
		if !known {
			owners = append(owners, ob)
		}
	}
	for _, ob := range owners {
		e.dispatch(ob)
	}
}

func (e *Engine) onViewed(ev *api.Event) {
	e.stats.Views++
	e.sawDriver(ev.DriverID, ev.At)
	if g := e.gigs[ev.GigID]; g != nil {
		g.views++
	}
}

func (e *Engine) onBooked(ev *api.Event) {
	g := e.gigs[ev.GigID]
	if g == nil {
		return
	}
	g.status, g.driver, g.fee = gigBooked, ev.DriverID, ev.FeeCents
	g.offers = nil
	e.sawDriver(ev.DriverID, ev.At)
	e.stats.booked(g, ev.At)
	for _, ref := range g.legs {
		ob := e.orders[ref.orderID]
		if ob == nil {
			continue
		}
		ob.bookings++
		if ob.firstBookedAt == 0 {
			ob.firstBookedAt = ev.At
		}
		e.claimLeg(ob, ref.leg, g)
	}
}

// claimLeg makes a booked gig the carrier of a car. If the engine had already posted the
// car again (the booking raced a repost), the newer open gig is taken out of the app.
func (e *Engine) claimLeg(ob *orderBook, leg int, g *gigBook) {
	lb := &ob.legs[leg]
	if lb.delivered || lb.gig == g {
		return
	}
	if other := lb.gig; other != nil && other.status == gigOpen && other.id != "" {
		other.status = gigWithdrawn
		e.stats.Duplicates++
		e.send(api.Action{Type: api.ActWithdraw, GigID: other.id}, nil)
	}
	lb.gig = g
}

// onReopened handles a dropped booking or a no-show: the gig is open again with its schedule.
func (e *Engine) onReopened(ev *api.Event, noShow bool) {
	if noShow {
		e.stats.NoShows++
	} else {
		e.stats.Drops++
	}
	g := e.gigs[ev.GigID]
	if g == nil {
		return
	}
	g.status, g.driver, g.fee = gigOpen, "", 0
	for _, ref := range g.legs {
		if ob := e.orders[ref.orderID]; ob != nil {
			ob.drops++
		}
	}
	e.replan(g)
}

func (e *Engine) onDriverLate(ev *api.Event) {
	e.stats.DriverLate++
	e.logf("LATE driver %s on %s, pickup at %d", ev.DriverID, ev.GigID, ev.PickupAt)
}

func (e *Engine) onExpired(ev *api.Event) {
	e.stats.Expired++
	g := e.gigs[ev.GigID]
	if g == nil {
		return
	}
	g.status = gigWithdrawn
	e.freeLegs(g)
}

func (e *Engine) onPickedUp(ev *api.Event) {
	if g := e.gigs[ev.GigID]; g != nil {
		e.driverAt(g.driver, ev.PlaceID, ev.At)
	}
	if ob := e.orders[ev.OrderID]; ob != nil && ev.Leg < len(ob.legs) {
		ob.legs[ev.Leg].pickedUp = true
	}
}

// onVisitFailed posts the car again without the cause of the failure.
func (e *Engine) onVisitFailed(ev *api.Event) {
	e.stats.FailedVisits[ev.Reason]++
	ob := e.orders[ev.OrderID]
	if ob == nil || ev.Leg >= len(ob.legs) {
		return
	}
	lb := &ob.legs[ev.Leg]
	if g := lb.gig; g != nil {
		e.dropLeg(g, legRef{orderID: ev.OrderID, leg: ev.Leg})
	}
	lb.gig = nil
	switch ev.Reason {
	case reasonNotReady, reasonClosed:
		lb.notBefore = e.now + e.P.RetryAfterMin
	case reasonLicence:
		lb.needManual = true
	case reasonNoPlates:
		lb.needPlates = true
	}
	e.dispatch(ob)
}

// dropLeg removes one car from a gig; a gig without cars is over.
func (e *Engine) dropLeg(g *gigBook, ref legRef) {
	rest := g.legs[:0:0]
	for _, l := range g.legs {
		if l != ref {
			rest = append(rest, l)
		}
	}
	g.legs = rest
	if len(rest) == 0 && g.live() {
		g.status = gigDone
	}
}

func (e *Engine) onDelivered(ev *api.Event) {
	ob := e.orders[ev.OrderID]
	if ob == nil || ev.Leg >= len(ob.legs) {
		return
	}
	lb := &ob.legs[ev.Leg]
	lb.delivered = true
	if g := e.gigs[ev.GigID]; g != nil {
		e.driverAt(g.driver, ev.PlaceID, ev.At)
	}
	if g := lb.gig; g != nil {
		e.dropLeg(g, legRef{orderID: ev.OrderID, leg: ev.Leg})
	}
	lb.gig = nil
	if ev.At > ob.order.Deadline {
		e.stats.LateLegs++
	}
	if ob.done() {
		e.stats.Delivered++
	}
}

func (e *Engine) onDeadlineClose(ev *api.Event) {
	e.stats.DeadlineWarnings++
	ob := e.orders[ev.OrderID]
	if ob == nil {
		return
	}
	ob.urgent = true
	ob.order.LastPickupAt = ev.LastPickupAt
	for _, g := range e.gigsOf(ob) {
		e.replan(g)
	}
	e.dispatch(ob)
}

func (e *Engine) onLedger(line *api.LedgerLine) {
	if line == nil {
		return
	}
	e.stats.Ledger[line.Kind] += line.Cents
	if line.OrderID == nil {
		return
	}
	ob := e.orders[*line.OrderID]
	if ob == nil {
		return
	}
	switch line.Kind {
	case "revenue":
		ob.paid = true
		e.countTrust(ob, false)
	case "late_refund", "undelivered_penalty":
		ob.refunded = true
		e.countTrust(ob, true)
	case "driver_fee", "failed_visit", "release_fee", "waiting", "fine":
		ob.feesPaid -= line.Cents
	}
}

// countTrust keeps each dealer's count of on-time orders as the dealer keeps it: an order
// paid without a refund adds one (up to 25), a late order halves the count.
func (e *Engine) countTrust(ob *orderBook, late bool) {
	d := e.market.dealer(ob.order.DealerID)
	switch {
	case late && ob.trustLate:
		return
	case late:
		if ob.trustCounted {
			d.trust = max(0, d.trust-1) // the revenue line came first and was counted
		}
		ob.trustLate = true
		d.trust /= 2
	case !ob.trustCounted && !ob.trustLate:
		ob.trustCounted = true
		d.trust = min(maxTrust, d.trust+1)
	}
}
