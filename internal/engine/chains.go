package engine

import (
	"math"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// Chains: the cars of two (or more) orders in one gig, each collected near where the one
// before is delivered. The driver saves the trip home after one car and the trip out to
// the next, so one driver moves several cars: more cars per driver, which is what the
// shortage is short of. A chain is only an attempt: if no driver takes it within
// ChainTryMin minutes it is split back into two gigs, while both cars still have time.

// chainPlan is a feasible chain: orders in the order their cars are moved.
type chainPlan struct {
	orders  []*orderBook
	startBy int // last minute to collect the first car and still be on time with every car
	hop     int // road minutes of the hops between the cars
	drive   int // road minutes with a car
	chance  float64
	from    int // the minute the attempt's clock starts: now, or the morning for a chain made at night
}

// The chance that a driver books a chain within its attempt, fitted on 3,545 chain
// attempts of e10 weeks (logistic; AUC 0.79 on held-out worlds, well calibrated). A chain
// that ends near where it started is a round trip for a driver; a long one-way pair is
// booked one time in five.
const (
	chainBias       = 2.4931
	chainPerDriveH  = -0.0844 // hours at the wheel
	chainPerBackH   = -0.3622 // road hours from the last delivery back to the first pickup
	chainPerHopH    = -0.0472
	chainToday      = -0.7645 // the first car is ready today
	chainPerAgeH    = -0.1678 // hours the older order has waited
	chainThursday   = -0.3120
	chainFriday     = -0.7091
	chainMonday     = 0.8321
	chainEndDrivers = 0.1248 // per log(1 + drivers living within 30 km of the last delivery)
	chainBackCapMin = 480
	chainAgeCapMin  = 240
)

// chainChanceOf is the model itself; weekday is 0 (Monday) to 6.
func chainChanceOf(driveMin, backMin, hopMin int, today bool, ageMin, weekday, endDrivers int) float64 {
	x := chainBias + chainPerDriveH*float64(driveMin)/cal.Hour + chainPerBackH*float64(min(backMin, chainBackCapMin))/cal.Hour +
		chainPerHopH*float64(hopMin)/cal.Hour + chainPerAgeH*float64(min(max(ageMin, 0), chainAgeCapMin))/cal.Hour +
		chainEndDrivers*math.Log1p(float64(endDrivers))
	if today {
		x += chainToday
	}
	switch weekday {
	case 0:
		x += chainMonday
	case 3:
		x += chainThursday
	case fridayIndex:
		x += chainFriday
	}
	return sigmoid(x)
}

// chainChance is the chance that a planned chain finds a driver within its attempt.
func (e *Engine) chainChance(plan chainPlan) float64 {
	first, last := plan.orders[0], plan.orders[len(plan.orders)-1]
	start, end := first.order.Legs[0].Pickup, last.order.Legs[0].Delivery.PlaceID
	oldest := e.now
	for _, ob := range plan.orders {
		oldest = min(oldest, ob.wonAt)
	}
	today := cal.DayOf(start.Window.From) <= cal.DayOf(e.now)
	return chainChanceOf(plan.drive, e.Map.Minutes(end, start.PlaceID), plan.hop, today, e.now-oldest,
		cal.Weekday(cal.DayOf(e.now)), e.driversNear(end, false))
}

// chainable reports whether an order may go into a chain: one car, not yet collected, no
// special handling, and chain attempts left.
func (e *Engine) chainable(ob *orderBook) bool {
	return ob.chainTries < e.P.ChainTries && e.chainFit(ob)
}

// chainFit is chainable without the limit on attempts: an order that is already in an
// open chain may stay in it when the chain grows.
func (e *Engine) chainFit(ob *orderBook) bool {
	if ob.cancelled || len(ob.legs) != 1 || ob.narrowed {
		return false
	}
	lb := &ob.legs[0]
	leg := &ob.order.Legs[0]
	return !lb.delivered && !lb.pickedUp && !lb.plain && !leg.Car.Premium && !leg.Pickup.PrivateAddress && !leg.Delivery.PrivateAddress
}

// chainTimes is the timing of a chain of single-car orders, each car collected and
// delivered before the next one.
type chainTimes struct {
	start   int // earliest minute to collect the first car (no car is reached before it is ready)
	startBy int // last minute to collect the first car and still be on time with every car
	hops    int // road minutes between the cars
	maxHop  int // the longest of those hops
	drive   int // road minutes with a car
}

func (e *Engine) chainTimes(seq []*orderBook) chainTimes {
	t := chainTimes{start: e.now, startBy: e.endsAt}
	offset := 0 // minutes from the first pickup to the pickup of the current car
	for i, ob := range seq {
		leg := &ob.order.Legs[0]
		if i > 0 {
			hop := e.Map.Minutes(seq[i-1].order.Legs[0].Delivery.PlaceID, leg.Pickup.PlaceID)
			offset += hop
			t.hops += hop
			t.maxHop = max(t.maxHop, hop)
		}
		ready := earliestPickup(leg, max(e.now, ob.legs[0].notBefore))
		last := e.lastPickupOf(ob, []int{0}) - e.buffer(ob, []int{0})
		// The driver leaves so as to reach each car when it is ready, not before.
		t.start = max(t.start, ready-offset)
		t.startBy = min(t.startBy, last-offset)
		d := e.driveMin(leg)
		offset += d
		t.drive += d
	}
	return t
}

// planChain checks whether the orders can go as one gig, in this order, with every car on
// time, and returns the plan.
func (e *Engine) planChain(seq []*orderBook) (chainPlan, bool) {
	t := e.chainTimes(seq)
	from := e.chainClock()
	if t.maxHop > e.P.ChainHopMin || t.drive+t.hops > multiCarDriveLimit || t.startBy-max(t.start, from) < e.P.ChainSlackMin {
		return chainPlan{}, false
	}
	// The first car must be collected while its site is open on the way to that plan.
	if cal.NextOpen(seq[0].order.Legs[0].Pickup.Hours, t.start) > t.startBy {
		return chainPlan{}, false
	}
	return chainPlan{orders: seq, startBy: t.startBy, hop: t.hops, drive: t.drive, from: from}, true
}

// Drivers hardly look at the app between nightFrom and nightTo: of the chains posted
// around midnight one in twenty-five was booked within its attempt, against six in ten of
// those posted in the morning. An attempt made in those hours also gets nightExtraMin more,
// because the first hours of the day are slow.
const (
	nightFrom     = 22 * cal.Hour
	nightTo       = 7 * cal.Hour
	nightExtraMin = 60
	// nightRoomMin is the room a single order's gig needs after the morning for its clock
	// to wait until then (NightClock).
	nightRoomMin = 360
)

// chainClock is the minute a chain attempt made now starts to count: now, or with
// ChainNight the next morning when it is night.
func (e *Engine) chainClock() int {
	if e.P.ChainNight != 1 {
		return e.now
	}
	return e.morning()
}

// morning is now in the hours when drivers look at the app, and the coming 07:00 at night.
func (e *Engine) morning() int {
	switch minute := cal.MinuteOfDay(e.now); {
	case minute >= nightFrom:
		return cal.At(cal.DayOf(e.now)+1, nightTo)
	case minute < nightTo:
		return cal.At(cal.DayOf(e.now), nightTo)
	}
	return e.now
}

// isChain reports whether a gig carries the single cars of more than one order.
func (e *Engine) isChain(g *gigBook) bool {
	if len(g.legs) < 2 {
		return false
	}
	for _, ref := range g.legs {
		ob := e.orders[ref.orderID]
		if ob == nil || ref.leg != 0 || len(ob.legs) != 1 || ref.orderID == g.legs[0].orderID && ref != g.legs[0] {
			return false
		}
	}
	return true
}

// adoptChain takes a chain found in the world after a restart: it stays an attempt that
// ends ChainTryMin after it was posted, and its orders have had their attempt.
func (e *Engine) adoptChain(g *gigBook) {
	seq := make([]*orderBook, 0, len(g.legs))
	for _, ref := range g.legs {
		ob := e.orders[ref.orderID]
		ob.chainTries = max(ob.chainTries, 1)
		seq = append(seq, ob)
	}
	g.mode = modeChain
	g.base = int(float64(g.base) * e.P.ChainFeeShare)
	g.chainLast = e.chainTimes(seq).startBy
	g.chainUntil = max(1, min(g.chainLast, g.postedAt+e.P.ChainTryMin))
	g.bookableUntil = g.chainUntil
}

// ordersOf lists the orders of an open gig in the order of its cars, or nil when one of
// them cannot be in a chain.
func (e *Engine) ordersOf(g *gigBook) []*orderBook {
	out := make([]*orderBook, 0, len(g.legs))
	for _, ref := range g.legs {
		ob := e.orders[ref.orderID]
		if ob == nil || ob.urgent || !e.chainFit(ob) || (g.mode != modeChain && !e.chainable(ob)) {
			return nil
		}
		out = append(out, ob)
	}
	return out
}

// findChain looks for an open gig that the order can be chained with: a single car, or a
// chain that still has room. The order goes before or after the gig's cars; the pairing
// with the shortest hops wins.
func (e *Engine) findChain(ob *orderBook) (plan chainPlan, partner *gigBook, ok, skipped bool) {
	var best chainPlan
	var bestGig *gigBook
	found := false
	for _, g := range append(e.openGigs(), e.unsentSingles()...) {
		single := g.mode == modeBuffered && len(g.legs) == 1
		grows := g.mode == modeChain && len(g.legs) < e.P.ChainMaxCars
		if (!single && !grows) || g.carries(legRef{orderID: ob.order.ID, leg: 0}) {
			continue
		}
		others := e.ordersOf(g)
		if len(others) == 0 {
			continue
		}
		before := append([]*orderBook{ob}, others...)
		after := append(append([]*orderBook{}, others...), ob)
		for _, seq := range [][]*orderBook{before, after} {
			plan, ok := e.planChain(seq)
			if !ok {
				continue
			}
			if e.P.ChainMinChance > 0 {
				// Only a chain that a driver is likely to take is worth the wait; the likeliest wins.
				if plan.chance = e.chainChance(plan); plan.chance < e.P.ChainMinChance {
					skipped = true
					continue
				}
				if !found || plan.chance > best.chance {
					best, bestGig, found = plan, g, true
				}
				continue
			}
			if !found || plan.hop < best.hop {
				best, bestGig, found = plan, g, true
			}
		}
	}
	return best, bestGig, found, skipped && !found
}

// tryChain posts the order in a chain with an open gig's cars when one fits. It reports
// whether it did; if not, the order is posted on its own as usual.
func (e *Engine) tryChain(ob *orderBook) bool {
	if e.P.Chains != 1 || e.Level < levelNetwork || !e.chainable(ob) || ob.urgent {
		return false
	}
	plan, partner, ok, skipped := e.findChain(ob)
	if skipped {
		e.stats.ChainsSkipped++
	}
	if !ok {
		return false
	}
	if partner.status == gigPending {
		// Its post has not left yet: it is taken back, and the app only ever sees the chain.
		if !e.unsend(partner) {
			return false
		}
		e.stats.Posts--
		e.stats.ChainsBatched++
	} else {
		// The partner gig leaves the app; all its cars go into the new chain.
		e.send(api.Action{Type: api.ActWithdraw, GigID: partner.id}, nil)
	}
	partner.status = gigWithdrawn
	e.postChain(plan, e.P.ChainTryMin)
	return true
}

// unsentSingles lists the single-car gigs posted in this step whose post is still in the
// outbox (ChainBatch): the app has not seen them, so one can become a chain's partner
// without a withdraw.
func (e *Engine) unsentSingles() []*gigBook {
	if e.P.ChainBatch != 1 {
		return nil
	}
	var out []*gigBook
	for i := range e.outbox {
		g := e.outbox[i].gig
		if g == nil || g.status != gigPending || g.mode != modeBuffered || len(g.legs) != 1 {
			continue
		}
		// Only a car that has never been in a chain: two cars whose chain just ended are
		// both posted in one step, and must not be put back together.
		if ob := e.orders[g.legs[0].orderID]; ob != nil && ob.chainTries == 0 {
			out = append(out, g)
		}
	}
	return out
}

// unsend takes a gig's post back from the outbox and reports whether it was there.
func (e *Engine) unsend(g *gigBook) bool {
	kept := make([]Pending, 0, len(e.outbox))
	found := false
	for _, p := range e.outbox {
		if p.gig == g && !found {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	e.outbox = kept
	return found
}

// recoverChain gives a chain that its driver gave back a new attempt of ChainRecoverMin
// minutes, when its cars still have the room a new chain needs: the gig leaves the app and
// the same cars are posted as a chain again, on a fee clock that starts over, as a car
// posted alone would. One driver wanted the chain, so another may; without this its first
// attempt is long over and the review takes the chain apart in the same minute. It
// reports whether the chain was posted again.
func (e *Engine) recoverChain(g *gigBook) bool {
	if e.P.ChainRecoverMin <= 0 || g.mode != modeChain || g.id == "" {
		return false
	}
	seq := e.ordersOf(g)
	if len(seq) < 2 {
		return false
	}
	plan, ok := e.planChain(seq)
	if !ok {
		return false
	}
	g.status = gigWithdrawn
	e.send(api.Action{Type: api.ActWithdraw, GigID: g.id}, nil)
	e.stats.ChainsRecovered++
	e.postChain(plan, e.P.ChainRecoverMin)
	return true
}

// postChain puts a chain in the app: each car collected and delivered, then the next. The
// attempt lasts tryMin minutes.
func (e *Engine) postChain(plan chainPlan, tryMin int) {
	g := &gigBook{status: gigPending, postedAt: e.now, mode: modeChain}
	audience := api.Audience{}
	stops := make([]api.StopRef, 0, 2*len(plan.orders))
	for _, ob := range plan.orders {
		leg, lb := &ob.order.Legs[0], &ob.legs[0]
		buffer := e.buffer(ob, []int{0})
		g.legs = append(g.legs, legRef{orderID: ob.order.ID, leg: 0})
		g.base += e.baseFee(ob, []int{0})
		lb.gig = g
		ob.chainTries++ // after ChainTries attempts the order goes alone
		if ob.chainAt == 0 {
			ob.chainAt = e.now // the first attempt: a later one does not reset the car's wait
		}
		audience.ManualLicence = audience.ManualLicence || lb.needManual
		pickup := api.StopRef{OrderID: ob.order.ID, Leg: 0, Kind: stopPickup, WithPlates: lb.needPlates, WithCharging: e.needsCharging(leg)}
		pickup.Window = e.retryWindow(leg, lb, false)
		delivery := api.StopRef{OrderID: ob.order.ID, Leg: 0, Kind: stopDelivery, Window: bufferedDelivery(leg, ob.deadlineOf(), buffer)}
		stops = append(stops, pickup, delivery)
	}
	g.base = int(float64(g.base) * e.P.ChainFeeShare)
	// The attempt ends early enough for every car to find a driver on its own.
	g.chainLast = plan.startBy
	g.chainUntil = min(plan.startBy, e.now+tryMin)
	if plan.from > e.now {
		// Made at night: the attempt and its fee clock count from the morning.
		g.clockFrom = plan.from
		g.chainUntil = min(plan.startBy, plan.from+tryMin+nightExtraMin)
	}
	g.plan = e.planFor(g, plan.startBy)
	g.shown = g.plan[0].FeeCents
	g.post = api.Action{Type: api.ActPost, Stops: stops, Audience: &audience, Schedule: e.visible(g.plan)}
	e.stats.Posts++
	e.stats.Chains++
	if len(plan.orders) > 2 {
		e.stats.LongChains++
	}
	e.sendPost(g, func(res api.ActionResult) { e.postedChain(g, res) })
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

// ghostOrder is the order a request would become if it were won now with the option. It
// is for planning only and never enters the books.
func (e *Engine) ghostOrder(req *api.PriceRequest, opt api.DeliveryOption) *orderBook {
	legs := make([]api.Leg, len(req.Legs))
	copy(legs, req.Legs)
	for i := range legs {
		// An order's windows end at the won option's deadline.
		if !legs[i].Pickup.PrivateAddress {
			legs[i].Pickup.Window.To = opt.Deadline
		}
		if !legs[i].Delivery.PrivateAddress {
			legs[i].Delivery.Window.To = opt.Deadline
		}
	}
	order := api.Order{ID: req.ID, DealerID: req.DealerID, Option: opt.Kind, Deadline: opt.Deadline, Legs: legs,
		DeliveredAt: make([]*int, len(legs)), LastPickupAt: e.latestPickup(&legs[0], opt.Deadline)}
	ob := &orderBook{order: order, req: req, legs: make([]legBook, len(legs)), wonAt: e.now}
	e.applyFacts(ob)
	return ob
}

// chainProspect is the chance that the request, won now with the option, goes into a
// chain that a driver takes: 0 when no open gig fits.
func (e *Engine) chainProspect(req *api.PriceRequest, opt api.DeliveryOption) float64 {
	if e.P.Chains != 1 || e.Level < levelNetwork || len(req.Legs) != 1 {
		return 0
	}
	ghost := e.ghostOrder(req, opt)
	if !e.chainable(ghost) {
		return 0
	}
	plan, _, ok, _ := e.findChain(ghost)
	if !ok {
		return 0
	}
	if plan.chance == 0 {
		plan.chance = e.chainChance(plan)
	}
	return plan.chance
}
