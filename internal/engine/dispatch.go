package engine

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// A refused post is tried again at once without windows; further refusals wait
// postRetryMin longer each time, up to maxPostBackoff steps.
const (
	postRetryMin   = 20
	maxPostBackoff = 6
)

// dispatch posts a gig for every undelivered car of the order that has none.
func (e *Engine) dispatch(ob *orderBook) {
	if ob.cancelled || e.replay || e.P.Research == 1 {
		return
	}
	var free []int
	for _, i := range ob.undelivered() {
		if lb := &ob.legs[i]; lb.gig == nil && !lb.pickedUp && e.now >= lb.retryAt {
			free = append(free, i)
		}
	}
	for _, group := range e.groupLegs(ob, free) {
		e.post(ob, group)
	}
}

// groupLegs decides which cars of one order share a gig: a trade-in goes as one gig
// from level 3 when its driving fits the 12-hour limit, else one gig per car.
func (e *Engine) groupLegs(ob *orderBook, legs []int) [][]int {
	if len(legs) == 2 && e.Level >= levelNetwork && e.tradeInDrive(ob) <= multiCarDriveLimit {
		return [][]int{legs}
	}
	groups := make([][]int, 0, len(legs))
	for _, i := range legs {
		groups = append(groups, []int{i})
	}
	return groups
}

// tradeInDrive is the road minutes of a two-car order as one gig, the drive between the cars included.
func (e *Engine) tradeInDrive(ob *orderBook) int {
	a, b := &ob.order.Legs[0], &ob.order.Legs[1]
	return e.Map.Minutes(a.Pickup.PlaceID, a.Delivery.PlaceID) +
		e.Map.Minutes(a.Delivery.PlaceID, b.Pickup.PlaceID) +
		e.Map.Minutes(b.Pickup.PlaceID, b.Delivery.PlaceID)
}

// baseFee is the typical fee of the given cars. The request's typical fee was set for the
// lanes the request named; it is shared between the order's cars by drive minutes and
// scaled to the lane each car has now, because a dealer can send a car somewhere else
// after the order is won, and a fee for the old, shorter lane finds no driver.
func (e *Engine) baseFee(ob *orderBook, legs []int) int {
	ref := ob.req.ReferenceFeeCents
	quoted, now := 0, 0
	for i := range ob.order.Legs {
		original := &ob.order.Legs[i]
		if i < len(ob.req.Legs) {
			original = &ob.req.Legs[i]
		}
		quoted += max(1, e.driveMin(original))
	}
	for _, i := range legs {
		now += max(1, e.driveMin(&ob.order.Legs[i]))
	}
	return max(minFeeCents, int(float64(ref)*float64(now)/float64(quoted)))
}

// lastPickupOf is the last on-time pickup of the first car of a gig: the world's value
// when the gig starts with the order's first undelivered car, else the engine's estimate.
func (e *Engine) lastPickupOf(ob *orderBook, legs []int) int {
	if len(legs) == 0 {
		return ob.order.LastPickupAt
	}
	own := e.latestPickup(&ob.order.Legs[legs[0]], ob.deadlineOf())
	if und := ob.undelivered(); len(und) > 0 && und[0] == legs[0] {
		if ob.noteDeadline > 0 || ob.narrowed {
			return min(ob.order.LastPickupAt, own) // the world's value knows neither
		}
		return ob.order.LastPickupAt
	}
	return own
}

func (e *Engine) lastPickupOfGig(g *gigBook) int {
	if len(g.legs) == 0 {
		return e.now
	}
	ob := e.orders[g.legs[0].orderID]
	if ob == nil {
		return e.now
	}
	legs := make([]int, 0, len(g.legs))
	for _, ref := range g.legs {
		if ref.orderID == ob.order.ID {
			legs = append(legs, ref.leg)
		}
	}
	return e.lastPickupOf(ob, legs)
}

// visible is the part of a fee plan the world accepts: one step at level 1.
func (e *Engine) visible(plan []api.FeeStep) []api.FeeStep {
	if e.Level < levelSchedules && len(plan) > 1 {
		return plan[:1]
	}
	return plan
}

// Posting modes: how much room the gig's windows leave.
const (
	modeBuffered = iota // delivery window ends a buffer before the deadline
	modeTight           // the order's own windows: on time only without the buffer
	modeLate            // no on-time pickup left: the world's windows run to its end
	modeDay             // real hours shorter than stated: both stops get one day's windows
)

// buffer is the minutes the delivery should arrive before the deadline, to absorb real driving times.
func (e *Engine) buffer(ob *orderBook, legs []int) int {
	drive := 0
	for _, i := range legs {
		drive += e.driveMin(&ob.order.Legs[i])
	}
	return max(e.P.BufferMin, int(e.P.BufferShare*float64(drive)))
}

// post puts one gig for the given cars of an order in the drivers' app.
func (e *Engine) post(ob *orderBook, legs []int) {
	lastPickup := e.lastPickupOf(ob, legs)
	buffer := e.buffer(ob, legs)
	base := e.baseFee(ob, legs)
	g := &gigBook{status: gigPending, base: base, postedAt: e.now, mode: modeLate}
	for _, i := range legs {
		g.legs = append(g.legs, legRef{orderID: ob.order.ID, leg: i})
	}
	first := max(e.now, earliestPickup(&ob.order.Legs[legs[0]], max(e.now, ob.legs[legs[0]].notBefore)))
	plain := false
	for _, i := range legs {
		plain = plain || ob.legs[i].plain
	}
	var dayPickup, dayDelivery api.TimeWindow
	dayOK := false
	if ob.narrowed && !plain && len(legs) == 1 && legs[0] == 0 {
		dayPickup, dayDelivery, g.dayLate, dayOK = e.dayWindows(ob, buffer)
	}
	switch {
	case dayOK:
		g.mode, g.bookableUntil = modeDay, dayPickup.To
	case plain:
		// A refused post goes again with the order's own windows only.
		if lastPickup > e.now && lastPickup >= first {
			g.mode = modeTight
		}
	case lastPickup-buffer >= first && lastPickup-buffer-e.now > e.P.FinalMin:
		g.mode, g.buffer = modeBuffered, buffer
	case lastPickup >= first && lastPickup > e.now:
		g.mode = modeTight
	}
	mode := g.mode
	g.plan = e.planFor(g, lastPickup)
	g.shown = g.plan[0].FeeCents
	audience := api.Audience{}
	stops := make([]api.StopRef, 0, 2*len(legs))
	for _, i := range legs {
		leg, lb := &ob.order.Legs[i], &ob.legs[i]
		lb.gig = g
		audience.Experienced = audience.Experienced || leg.Car.Premium
		audience.ManualLicence = audience.ManualLicence || lb.needManual
		pickup := api.StopRef{OrderID: ob.order.ID, Leg: i, Kind: stopPickup, WithPlates: lb.needPlates, WithCharging: e.needsCharging(leg)}
		if !plain {
			pickup.Window = e.retryWindow(leg, lb, mode == modeLate)
		}
		delivery := api.StopRef{OrderID: ob.order.ID, Leg: i, Kind: stopDelivery}
		switch {
		case mode == modeDay:
			pickup.Window, delivery.Window = &dayPickup, &dayDelivery
		case plain:
		case mode == modeBuffered:
			delivery.Window = bufferedDelivery(leg, ob.deadlineOf(), buffer)
		case mode == modeTight && ob.deadlineOf() < ob.order.Deadline:
			// The order's own window runs to its deadline; the note's is earlier.
			delivery.Window = bufferedDelivery(leg, ob.deadlineOf(), 0)
		}
		stops = append(stops, pickup, delivery)
	}
	g.post = api.Action{Type: api.ActPost, Stops: stops, Audience: &audience, Schedule: e.visible(g.plan)}
	e.stats.Posts++
	e.send(g.post, func(res api.ActionResult) { e.posted(ob, g, res) })
}

// bufferedDelivery is the delivery stop's own window: it ends a buffer before the last
// minute the site takes the car by the deadline. Nil at a private address (a booked hour).
func bufferedDelivery(leg *api.Leg, deadline, buffer int) *api.TimeWindow {
	if leg.Delivery.PrivateAddress {
		return nil
	}
	arrive := deadline
	if open := cal.LastOpen(leg.Delivery.Hours, deadline); open >= 0 {
		arrive = open
	}
	to := arrive - buffer
	if to <= leg.Delivery.Window.From {
		return nil
	}
	return &api.TimeWindow{From: leg.Delivery.Window.From, To: to}
}

// repost takes an open gig out of the app and posts its cars again in the mode that fits now.
// Both actions go in one batch. If the withdraw is refused the gig had already left the
// open state (expired in that minute, or a driver took it); the events say which, and
// onBooked settles a booking that raced the repost.
func (e *Engine) repost(g *gigBook) {
	if g.status != gigOpen || g.id == "" {
		return
	}
	g.status = gigWithdrawn
	e.stats.Reposts++
	e.send(api.Action{Type: api.ActWithdraw, GigID: g.id}, nil)
	e.freeLegs(g)
}

// retryWindow is the pickup's own window after a failed visit: not before the retry time.
// Nil keeps the order's window. A private seller's slot never extends, so a retry past
// it gets no window of its own (the dealer books a new slot, seen as order_changed).
func (e *Engine) retryWindow(leg *api.Leg, lb *legBook, late bool) *api.TimeWindow {
	if lb.notBefore <= leg.Pickup.Window.From {
		return nil
	}
	from, to := lb.notBefore, leg.Pickup.Window.To
	if leg.Pickup.PrivateAddress {
		if from > to {
			return nil
		}
		return &api.TimeWindow{From: from, To: to}
	}
	if late || to <= from {
		to = e.endsAt
	}
	return &api.TimeWindow{From: from, To: to}
}

// posted records the world's answer to a post.
func (e *Engine) posted(ob *orderBook, g *gigBook, res api.ActionResult) {
	if res.OK {
		g.id = res.ID
		e.gigs[g.id] = g
		if g.status != gigPending {
			// Its cars found another carrier while the post was on its way.
			e.send(api.Action{Type: api.ActWithdraw, GigID: g.id}, nil)
			return
		}
		g.status = gigOpen
		return
	}
	g.status = gigWithdrawn
	e.stats.PostFailures++
	for _, ref := range g.legs {
		lb := &ob.legs[ref.leg]
		if lb.gig == g {
			lb.gig = nil
		}
		lb.fails++
		// The next try carries no window of its own (the world always takes that), and
		// waits longer each time; the review brings the car back, so it is never dropped.
		lb.plain, lb.notBefore = true, 0
		if lb.fails > 1 {
			lb.retryAt = e.now + postRetryMin*min(lb.fails-1, maxPostBackoff)
		}
	}
	e.logf("post of %s refused (%s); it goes again without windows", ob.order.ID, res.Reason)
	e.dispatch(ob)
}

// reprice puts the rest of the gig's fee plan in the app.
func (e *Engine) reprice(g *gigBook) {
	if g.status != gigOpen || g.id == "" {
		return
	}
	schedule := e.visible(fromNow(g.plan, e.now))
	g.shown = schedule[0].FeeCents
	e.stats.Reprices++
	e.send(api.Action{Type: api.ActReprice, GigID: g.id, Schedule: schedule}, nil)
}

// planFor is the fee plan of a gig from now, by its posting mode. It also sets until
// when drivers can still book the gig inside its windows.
func (e *Engine) planFor(g *gigBook, lastPickup int) []api.FeeStep {
	p := e.P
	p.FeeMax *= e.feeBoost(g)
	price := e.priceOf(g)
	onTimeCap, lateCap := int(e.P.OnTimePriceCap*float64(price)), int(e.P.LatePriceCap*float64(price))
	switch g.mode {
	case modeBuffered:
		g.bookableUntil = lastPickup - g.buffer
		return capSchedule(p.Schedule(e.now, g.bookableUntil, g.base), onTimeCap)
	case modeTight:
		g.bookableUntil = lastPickup
		return capSchedule([]api.FeeStep{{At: e.now, FeeCents: feeOf(g.base, p.FeeMax)}}, onTimeCap)
	case modeDay:
		// bookableUntil is the end of the day's pickup window, set when the gig was posted.
		if g.dayLate {
			return capSchedule(p.lateSchedule(e.now, g.base, e.endsAt, e.lastFee(g)), lateCap)
		}
		return capSchedule(p.Schedule(e.now, g.bookableUntil, g.base), onTimeCap)
	}
	g.bookableUntil = 0
	return capSchedule(p.lateSchedule(e.now, g.base, e.endsAt, e.lastFee(g)), lateCap)
}

// priceOf is the price of the orders a gig carries.
func (e *Engine) priceOf(g *gigBook) int {
	price := 0
	seen := map[string]bool{}
	for _, ref := range g.legs {
		if ob := e.orders[ref.orderID]; ob != nil && !seen[ref.orderID] {
			seen[ref.orderID] = true
			price += ob.order.PriceCents
		}
	}
	return price
}

// lastFee is the most a late gig may come to pay before the world ends: LastMult times
// its base fee, capped by a share of the price of the orders it carries.
func (e *Engine) lastFee(g *gigBook) int {
	return min(feeOf(g.base, e.P.LastMult), int(e.P.LastPriceShare*float64(e.priceOf(g))))
}

// feeBoost raises the top fee of a gig for a trusted dealer: being late there costs the
// refund and half the dealer's trust.
func (e *Engine) feeBoost(g *gigBook) float64 {
	if e.Level < levelTrust || e.P.TrustFeeBoost <= 1 || len(g.legs) == 0 {
		return 1
	}
	ob := e.orders[g.legs[0].orderID]
	if ob == nil {
		return 1
	}
	trust := float64(e.market.dealer(ob.order.DealerID).trust) / maxTrust
	return 1 + (e.P.TrustFeeBoost-1)*trust
}

// replan rebuilds an open gig's fee plan from now, for a last pickup that may have moved.
// The fee never falls below what the app shows now.
func (e *Engine) replan(g *gigBook) {
	if g.status != gigOpen {
		return
	}
	current := fromNow(g.plan, e.now)
	fresh := floorSchedule(e.planFor(g, e.lastPickupOfGig(g)), current[0].FeeCents)
	if sameSchedule(current, fresh) {
		return
	}
	g.plan = fresh
	e.reprice(g)
}
