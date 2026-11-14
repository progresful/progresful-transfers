package engine

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// Chains: two cars of two orders in one gig, the second collected near where the first
// is delivered. The driver saves the trip home after the first car and the trip out to
// the second, so one driver moves two cars: more cars per driver, which is what the
// shortage is short of. A chain is only an attempt: if no driver takes it within
// ChainTryMin minutes it is split back into two gigs, while both cars still have time.

// chainPlan is a feasible chain of two single-car orders.
type chainPlan struct {
	first, second *orderBook
	startBy       int // last minute to collect the first car and still be on time with both
	hop           int // road minutes from the first delivery to the second pickup
}

// chainable reports whether an order may go into a chain: one car, not yet collected, no
// special handling, and not already tried in a chain.
func (e *Engine) chainable(ob *orderBook) bool {
	if ob.cancelled || len(ob.legs) != 1 || ob.noChain || ob.narrowed {
		return false
	}
	lb := &ob.legs[0]
	leg := &ob.order.Legs[0]
	return !lb.delivered && !lb.pickedUp && !lb.plain && !leg.Car.Premium && !leg.Pickup.PrivateAddress && !leg.Delivery.PrivateAddress
}

// planChain checks whether first then second can go as one gig with both cars on time,
// and returns the plan.
func (e *Engine) planChain(first, second *orderBook) (chainPlan, bool) {
	a, b := &first.order.Legs[0], &second.order.Legs[0]
	hop := e.Map.Minutes(a.Delivery.PlaceID, b.Pickup.PlaceID)
	driveA, driveB := e.driveMin(a), e.driveMin(b)
	if hop > e.P.ChainHopMin || driveA+hop+driveB > multiCarDriveLimit {
		return chainPlan{}, false
	}
	lastA := e.lastPickupOf(first, []int{0}) - e.buffer(first, []int{0})
	lastB := e.lastPickupOf(second, []int{0}) - e.buffer(second, []int{0})
	readyA := earliestPickup(a, max(e.now, first.legs[0].notBefore))
	readyB := earliestPickup(b, max(e.now, second.legs[0].notBefore))
	// The driver leaves so as to reach the second car when it is ready, not before.
	start := max(readyA, readyB-driveA-hop)
	startBy := min(lastA, lastB-driveA-hop)
	if startBy-start < e.P.ChainSlackMin {
		return chainPlan{}, false
	}
	// The first car must be collected while its site is open on the way to that plan.
	if cal.NextOpen(a.Pickup.Hours, start) > startBy {
		return chainPlan{}, false
	}
	return chainPlan{first: first, second: second, startBy: startBy, hop: hop}, true
}

// findChain looks for an open single-car gig that the order can be chained with, in either
// order, and returns the tightest pairing (the shortest hop).
func (e *Engine) findChain(ob *orderBook) (chainPlan, *gigBook, bool) {
	var best chainPlan
	var bestGig *gigBook
	found := false
	for _, g := range e.openGigs() {
		if g.mode != modeBuffered || len(g.legs) != 1 || g.legs[0].orderID == ob.order.ID {
			continue
		}
		other := e.orders[g.legs[0].orderID]
		if other == nil || !e.chainable(other) || other.urgent {
			continue
		}
		for _, pair := range [2][2]*orderBook{{ob, other}, {other, ob}} {
			plan, ok := e.planChain(pair[0], pair[1])
			if ok && (!found || plan.hop < best.hop) {
				best, bestGig, found = plan, g, true
			}
		}
	}
	return best, bestGig, found
}

// tryChain posts the order in a chain with an open gig's car when one fits. It reports
// whether it did; if not, the order is posted on its own as usual.
func (e *Engine) tryChain(ob *orderBook) bool {
	if e.P.Chains != 1 || e.Level < levelNetwork || !e.chainable(ob) || ob.urgent {
		return false
	}
	plan, partner, ok := e.findChain(ob)
	if !ok {
		return false
	}
	// The partner's own gig leaves the app; both cars go into the chain.
	partner.status = gigWithdrawn
	e.send(api.Action{Type: api.ActWithdraw, GigID: partner.id}, nil)
	e.postChain(plan)
	return true
}

// postChain puts a two-car gig in the app: first car out and delivered, then the second.
func (e *Engine) postChain(plan chainPlan) {
	orders := [2]*orderBook{plan.first, plan.second}
	g := &gigBook{status: gigPending, postedAt: e.now, mode: modeChain}
	audience := api.Audience{}
	stops := make([]api.StopRef, 0, 4)
	for _, ob := range orders {
		leg, lb := &ob.order.Legs[0], &ob.legs[0]
		buffer := e.buffer(ob, []int{0})
		g.legs = append(g.legs, legRef{orderID: ob.order.ID, leg: 0})
		g.base += e.baseFee(ob, []int{0})
		lb.gig = g
		ob.noChain = true // one attempt per order: after this it goes alone
		audience.ManualLicence = audience.ManualLicence || lb.needManual
		pickup := api.StopRef{OrderID: ob.order.ID, Leg: 0, Kind: stopPickup, WithPlates: lb.needPlates, WithCharging: e.needsCharging(leg)}
		pickup.Window = e.retryWindow(leg, lb, false)
		delivery := api.StopRef{OrderID: ob.order.ID, Leg: 0, Kind: stopDelivery, Window: bufferedDelivery(leg, ob.deadlineOf(), buffer)}
		stops = append(stops, pickup, delivery)
	}
	g.base = int(float64(g.base) * e.P.ChainFeeShare)
	// The attempt ends early enough for both cars to find a driver on their own.
	g.chainLast = plan.startBy
	g.chainUntil = min(plan.startBy, e.now+e.P.ChainTryMin)
	g.plan = e.planFor(g, plan.startBy)
	g.shown = g.plan[0].FeeCents
	g.post = api.Action{Type: api.ActPost, Stops: stops, Audience: &audience, Schedule: e.visible(g.plan)}
	e.stats.Posts++
	e.stats.Chains++
	e.send(g.post, func(res api.ActionResult) { e.postedChain(g, res) })
}

// postedChain records the world's answer to a chain post. A refused chain frees its cars,
// and the review posts them again one by one.
func (e *Engine) postedChain(g *gigBook, res api.ActionResult) {
	if res.OK {
		g.id = res.ID
		e.gigs[g.id] = g
		intact := g.status == gigPending
		for _, ref := range g.legs {
			ob := e.orders[ref.orderID]
			intact = intact && ob != nil && ob.legs[ref.leg].gig == g
		}
		if !intact {
			// A car found another carrier while the post was on its way: no chain.
			g.status = gigWithdrawn
			e.send(api.Action{Type: api.ActWithdraw, GigID: g.id}, nil)
			e.freeLegs(g)
			return
		}
		g.status = gigOpen
		return
	}
	g.status = gigWithdrawn
	e.stats.PostFailures++
	e.logf("chain refused (%s); its cars go alone", res.Reason)
	e.freeLegs(g)
}
