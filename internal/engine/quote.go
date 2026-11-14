package engine

import (
	"math"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

const (
	chargingMin        = 40
	platesMin          = 30
	lowChargePercent   = 30
	chargingRouteKm    = 200.0
	multiCarDriveLimit = 12 * cal.Hour
)

// needsCharging applies handling rule 3: an electric car below 30% on a route over 200 km.
func (e *Engine) needsCharging(leg *api.Leg) bool {
	c := leg.Car
	if !c.Electric || c.ChargePercent == nil || *c.ChargePercent >= lowChargePercent {
		return false
	}
	return e.Map.Km(leg.Pickup.PlaceID, leg.Delivery.PlaceID) > chargingRouteKm
}

// driveMin is the planned minutes from pickup to delivery, with a charging stop when needed.
func (e *Engine) driveMin(leg *api.Leg) int {
	minutes := e.Map.Minutes(leg.Pickup.PlaceID, leg.Delivery.PlaceID)
	if e.needsCharging(leg) {
		minutes += chargingMin
	}
	return minutes
}

// earliestPickup is the first minute from `from` at which the car can be collected.
func earliestPickup(leg *api.Leg, from int) int {
	t := max(from, leg.Pickup.Window.From)
	if leg.Pickup.PrivateAddress {
		return t
	}
	if open := cal.NextOpen(leg.Pickup.Hours, t); open >= 0 {
		return open
	}
	return t
}

// privateGraceMin: a private delivery is due this long after its booked hour ends (handling rule 5).
const privateGraceMin = 30

// legDeadline is the minute by which one car must arrive: the order's deadline, or for a
// delivery to a private address the end of its booked hour plus the grace, if earlier.
// In a trade-in the new car goes to the customer's hour while the order's deadline is
// for the old car's way back.
func legDeadline(leg *api.Leg, deadline int) int {
	if leg.Delivery.PrivateAddress {
		return min(deadline, leg.Delivery.Window.To+privateGraceMin)
	}
	return deadline
}

// sameVisit reports whether car i of a request is collected in the visit that delivers
// the car before it: a trade-in's old car, taken at the customer's address when the new
// car arrives. In one gig its booked slot limits nothing (RULES.md, Gigs), so only the
// car before it decides whether the order can be on time.
func (e *Engine) sameVisit(legs []api.Leg, i int) bool {
	if i == 0 || i >= len(legs) || e.Level < levelNetwork || e.P.SplitTradeIns == 1 {
		return false
	}
	prev, leg := &legs[i-1], &legs[i]
	if !leg.Pickup.PrivateAddress || leg.Pickup.PlaceID != prev.Delivery.PlaceID {
		return false
	}
	drive := e.Map.Minutes(prev.Pickup.PlaceID, prev.Delivery.PlaceID) + e.Map.Minutes(prev.Delivery.PlaceID, leg.Pickup.PlaceID) + e.Map.Minutes(leg.Pickup.PlaceID, leg.Delivery.PlaceID)
	return drive <= multiCarDriveLimit
}

// latestPickup is the last minute to collect the car and still deliver it by the deadline,
// by the stated hours of both sites. It is the engine's own estimate of lastPickupAt.
func (e *Engine) latestPickup(leg *api.Leg, deadline int) int {
	arrive := legDeadline(leg, deadline)
	if !leg.Delivery.PrivateAddress {
		if open := cal.LastOpen(leg.Delivery.Hours, deadline); open >= 0 {
			arrive = open
		}
	}
	latest := arrive - e.driveMin(leg)
	if leg.Pickup.PrivateAddress {
		return min(latest, leg.Pickup.Window.To)
	}
	if open := cal.LastOpen(leg.Pickup.Hours, latest); open >= 0 {
		return open
	}
	return latest
}

// slack is the minutes between the earliest pickup and the last pickup that still arrives
// a buffer before the deadline, for the tightest car of the request and one delivery option.
func (e *Engine) slack(req *api.PriceRequest, opt api.DeliveryOption) int {
	worst := math.MaxInt32
	ready, deadline := e.effective(req, opt)
	for i := range req.Legs {
		if e.sameVisit(req.Legs, i) {
			continue
		}
		leg := &req.Legs[i]
		from, by := req.RespondBy, opt.Deadline
		if i == 0 {
			from, by = ready, deadline // the note speaks of the first car
			leg = e.realLeg(req)
		}
		buffer := e.bufferOf(leg, by)
		worst = min(worst, e.latestPickup(leg, by)-buffer-earliestPickup(leg, from))
	}
	return worst
}

// lead is the minutes from the dealer's answer to the last pickup of the first car that
// is still on time with the option: the time there is to find a driver.
func (e *Engine) lead(req *api.PriceRequest, opt api.DeliveryOption) int {
	_, deadline := e.effective(req, opt)
	return e.latestPickup(e.realLeg(req), deadline) - max(req.RespondBy, e.now)
}

// shortLead reports whether the option leaves too little lead for its drive.
func (e *Engine) shortLead(req *api.PriceRequest, opt api.DeliveryOption) bool {
	if e.P.MinLeadMin <= 0 && e.P.MinLeadDrive <= 0 {
		return false
	}
	need := float64(e.P.MinLeadMin) + e.P.MinLeadDrive*float64(e.driveMin(e.realLeg(req)))
	return float64(e.lead(req, opt)) < need
}

// supply counts the drivers who hear of a gig or find it on their way home: those living
// within the notification radius of the pickup or of the delivery, whichever has more.
func (e *Engine) supply(leg *api.Leg) int {
	experienced := leg.Car.Premium
	return max(e.driversNear(leg.Pickup.PlaceID, experienced), e.driversNear(leg.Delivery.PlaceID, experienced))
}

func (e *Engine) driversNear(place string, experienced bool) int {
	key := place
	if experienced {
		key += "|exp"
	}
	if n, ok := e.reach[key]; ok {
		return n
	}
	n := 0
	for _, d := range e.drivers {
		if experienced && (d.profile.Rating < experiencedRating || d.profile.CompletedGigs < experiencedGigs) {
			continue
		}
		if e.Map.AirKm(d.profile.HomePlaceID, place) <= notifyRadiusKm {
			n++
		}
	}
	e.reach[key] = n
	return n
}

// supplied reports whether enough drivers are in reach of every car of the request.
func (e *Engine) supplied(req *api.PriceRequest) bool {
	for i := range req.Legs {
		leg := &req.Legs[i]
		need := e.P.MinSupply
		if leg.Car.Premium {
			need = e.P.MinExperienced
		}
		if e.supply(leg) < need {
			return false
		}
	}
	return true
}

// costMult is the cost of an order as a multiple of its typical fee, today: CostMult,
// moved by how the day's wins compare with what the world's drivers can carry.
func (e *Engine) costMult() float64 {
	if e.cold && e.P.JoinCost > 0 {
		return e.P.JoinCost
	}
	if e.P.LoosenCost > 0 {
		// Drivers to spare: driver time is worth less, so orders cost less.
		return math.Max(e.P.CostMin, e.P.CostMult-e.P.LoosenCost*e.spare(e.onTimeOutlook()))
	}
	if e.P.TargetPerDriver <= 0 || len(e.drivers) == 0 {
		return e.P.CostMult
	}
	perDay := e.P.TargetPerDriver * float64(len(e.drivers)) / requestDays
	day, minute := cal.DayOf(e.now), cal.MinuteOfDay(e.now)
	elapsed := math.Max(0, math.Min(1, float64(minute-requestDayStart)/float64(requestDayEnd-requestDayStart)))
	won := 0
	if day == e.winsDay {
		won = e.winsToday
	}
	behind := (perDay*elapsed - float64(won)) / perDay
	return math.Max(e.P.CostMin, math.Min(e.P.CostMax, e.P.CostMult*math.Exp(-e.P.CostGain*behind)))
}

const (
	longLaneKm  = 300.0
	shortLaneKm = 60.0
)

// laneCost is the factor on the cost of an order for the length of its first car's lane.
func (e *Engine) laneCost(req *api.PriceRequest) float64 {
	km := e.Map.Km(req.Legs[0].Pickup.PlaceID, req.Legs[0].Delivery.PlaceID)
	switch {
	case km >= longLaneKm && e.P.CostLong > 0:
		return e.P.CostLong
	case km < shortLaneKm && e.P.CostShort > 0:
		return e.P.CostShort
	}
	return 1
}

// readyCost is the factor on the cost of an order for when its first car is ready: today
// or on a later day (after its note).
func (e *Engine) readyCost(req *api.PriceRequest, ref api.DeliveryOption) float64 {
	ready, _ := e.effective(req, ref)
	if cal.DayOf(ready) > cal.DayOf(req.ArrivedAt) {
		return e.P.CostLater
	}
	return e.P.CostToday
}

// dayCost is the weekday's factor on the cost of an order: Friday's drivers are dearer.
func (e *Engine) dayCost() float64 {
	if cal.DayOf(e.now) == fridayIndex && e.P.FridayCost > 0 {
		return e.P.FridayCost
	}
	return 1
}

func hasPremium(req *api.PriceRequest) bool {
	for i := range req.Legs {
		if req.Legs[i].Car.Premium {
			return true
		}
	}
	return false
}

// costFloor is the least price that leaves the wanted margin over the driver's fee. Up to
// level 3 (list prices) it is the bar a request must clear to be taken at all.
func (e *Engine) costFloor(req *api.PriceRequest) int {
	mult := e.P.FloorMult
	if e.Level < levelQuotes {
		mult = e.P.CostMult
	}
	cost := float64(req.ReferenceFeeCents) * mult
	margin := math.Max(float64(e.P.MinMarginCents), e.P.MinMarginShare*cost)
	return int(math.Ceil(cost + margin))
}

// shareMinRequests is how many requests a dealer must have sent before its won share counts.
const shareMinRequests = 12

// pendingQuote remembers what was quoted, to learn from the dealer's answer.
type pendingQuote struct {
	Dealer string  `json:"dealer"`
	Place  string  `json:"place"` // pickup place of the first car
	Day    int     `json:"day"`   // day the quote was made
	Dest   string  `json:"dest"`  // delivery place of the first car
	Level  float64 `json:"level"` // the market level the quote was priced against
	Ratio  float64 `json:"ratio"` // quoted price over list price
}

// marketStartFor is the first guess of the winning ratio at a level: the middle of the
// competitor's published price level, a little under it.
func marketStartFor(level int) float64 {
	switch {
	case level >= 8:
		return 1.32
	case level == 7:
		return 1.18
	}
	return 0.97
}

func (e *Engine) onRequest(req *api.PriceRequest) {
	if req == nil || len(req.Options) == 0 {
		return
	}
	if _, seen := e.reqs[req.ID]; !seen {
		e.stats.Requests++
		e.market.dealer(req.DealerID).requests++
		e.listSum += req.Options[0].ListPriceCents
		e.listCount++
	}
	e.reqs[req.ID] = req
	if e.replay && e.P.ReplayQuotes != 1 {
		// A request of the world's past: it was answered, or not, back then. Pricing it
		// now would make the dealer's old answer look like an answer to this price.
		return
	}
	e.why, e.whyRisk, e.whyCap = "", 0, 0
	prices := e.pricesFor(req)
	if !e.replay {
		d := Decision{At: e.now, RequestID: req.ID, Risk: e.whyRisk, Cap: e.whyCap}
		if len(prices) == 0 {
			d.Why = e.why
		}
		e.decided = append(e.decided, d)
	}
	if len(prices) == 0 {
		e.stats.Declined++
		if e.Level < levelQuotes {
			return // every quote wins up to level 3: declining is sending none
		}
		first := req.Options[0]
		prices = map[string]int{first.Kind: int(float64(first.ListPriceCents) * e.P.DeclineMult)}
	} else {
		e.stats.Quoted++
	}
	e.send(api.Action{Type: api.ActQuote, RequestID: req.ID, Prices: prices}, nil)
}

// pricesFor returns the price of each option worth offering; empty means decline.
func (e *Engine) pricesFor(req *api.PriceRequest) map[string]int {
	if !e.supplied(req) {
		e.stats.DeclinedSupply++
		e.why = whyFewDrivers
		return nil
	}
	if why := e.noteRefusal(req); why != "" {
		e.stats.DeclinedNote[why]++
		e.why = why
		return nil
	}
	if e.P.Premium != 1 && hasPremium(req) {
		e.stats.DeclinedNote["premium car"]++
		e.why = whyPremium
		return nil
	}
	if e.P.TradeIns != 1 && len(req.Legs) > 1 && e.Level >= levelQuotes {
		e.stats.DeclinedNote["trade-in"]++
		e.why = whyTradeIn
		return nil
	}
	floor := e.costFloor(req)
	if e.Level < levelQuotes {
		// The dealer ignores the price and takes the first option at its list price.
		first := req.Options[0]
		if first.ListPriceCents < floor {
			e.why = whyNoMargin
			return nil
		}
		if e.slack(req, first) < e.P.MinSlackMin || e.shortLead(req, first) {
			e.why = whyNoTime
			return nil
		}
		return map[string]int{first.Kind: first.ListPriceCents}
	}
	var offered []api.DeliveryOption
	risk, riskCap := 0.0, e.riskCap(e.market.dealer(req.DealerID))
	safest := math.Inf(1) // the lowest risk among the options refused for their risk
	for _, opt := range req.Options {
		if e.slack(req, opt) < e.P.MinSlackMin || e.shortLead(req, opt) {
			continue
		}
		r := e.lateRisk(req, opt)
		if r > riskCap {
			e.stats.RiskyOptions++
			safest = math.Min(safest, r)
			continue
		}
		offered = append(offered, opt)
		risk = math.Max(risk, r)
	}
	e.whyCap = riskCap
	if len(offered) == 0 {
		e.why = whyNoTime
		if !math.IsInf(safest, 1) {
			e.why, e.whyRisk = whyRisky, safest
		}
		return nil
	}
	if keep := e.P.SlowOptions; keep > 0 && len(offered) > keep {
		// Options come fastest first: keep the slowest ones, and price on their risk alone.
		offered = offered[len(offered)-keep:]
		risk = 0
		for _, opt := range offered {
			risk = math.Max(risk, e.lateRisk(req, opt))
		}
	}
	e.whyRisk = risk
	ratio := e.quoteRatio(req, offered[0], floor, risk)
	if ratio <= 0 {
		e.why = whyNoPrice
		return nil
	}
	prices := make(map[string]int, len(offered))
	for _, opt := range offered {
		r := ratio
		if opt.Kind == api.SameDay {
			r *= e.P.SameDayPremium
		}
		prices[opt.Kind] = max(floor, int(math.Round(float64(opt.ListPriceCents)*r)))
	}
	return prices
}

// quoteRatio is the price over list price to quote for a request, or 0 to decline.
// ref is the option whose list price the cost is compared with; risk is the chance
// that the order arrives late.
func (e *Engine) quoteRatio(req *api.PriceRequest, ref api.DeliveryOption, floor int, risk float64) float64 {
	list := float64(ref.ListPriceCents)
	d := e.market.dealer(req.DealerID)
	place, dest, day := req.Legs[0].Pickup.PlaceID, req.Legs[0].Delivery.PlaceID, cal.DayOf(e.now)
	level, width := e.priceLevel(req, d, day)
	ratio := e.P.PriceMult
	if e.P.Adaptive == 1 {
		mult := e.costMultFor(req, ref) + e.P.RiskSlope*risk
		cost := float64(req.ReferenceFeeCents)*mult*e.dayCost()*e.laneCost(req)*e.readyCost(req, ref) + risk*(e.P.RiskCost*list+e.P.TrustGuard*e.trustAtStake(d))
		var chance float64
		ratio, chance = bestRatioAt(level, width, cost/list, e.trustWorth(d)/list, float64(floor)/list)
		if chance < e.P.MinWinChance {
			return 0 // the price that pays is far above what this dealer accepts
		}
	}
	if d.requests >= shareMinRequests && float64(d.wins) > e.P.ShareCap*float64(d.requests) {
		ratio *= 1 + e.P.ShareMarkup
	}
	e.quotes[req.ID] = pendingQuote{Dealer: req.DealerID, Place: place, Dest: dest, Day: day, Level: level, Ratio: math.Max(ratio, float64(floor)/list)}
	return ratio
}

// priceLevel is the log ratio at which a quote for the request wins half the time, and
// the width of the win curve around it.
func (e *Engine) priceLevel(req *api.PriceRequest, d *dealerBook, day int) (float64, float64) {
	place, dest := req.Legs[0].Pickup.PlaceID, req.Legs[0].Delivery.PlaceID
	if b := e.market.beliefs; b != nil {
		level, width := b.level(req.DealerID, place, dest, day)
		return level + e.market.trustShift(d, e.P.TrustBoost), width
	}
	level := e.market.level(d, e.P.TrustBoost) + e.market.todayShift(req.DealerID, place, day) +
		e.P.LaneWeight*laneShift(place, dest) + e.market.destShift(dest)
	return level, e.market.scale
}

// costMultFor is the cost of carrying a request as a multiple of its typical fee: the
// day's scarcity cost, less for a request that would share a driver with an open car.
func (e *Engine) costMultFor(req *api.PriceRequest, ref api.DeliveryOption) float64 {
	mult := e.costMult()
	if e.P.CostByGig == 1 && e.Level >= levelRiskTrained {
		mult = e.expectedFeeMult(&req.Legs[0], req.RespondBy, ref.Kind) + e.P.CostSpread
	}
	if e.P.ChainCostGain > 0 {
		if prospect := e.chainProspect(req, ref); prospect > 0 {
			e.stats.ChainProspects++
			mult -= e.P.ChainCostGain * prospect
		}
	}
	return mult
}

// trustWorth is the extra worth (cents) of winning an order from a dealer whose trust can
// still grow: each on-time order lifts the price of the dealer's later orders by 1%.
func (e *Engine) trustWorth(d *dealerBook) float64 {
	if e.Level < levelTrust || e.P.TrustInvest <= 0 || d.trust >= maxTrust {
		return 0
	}
	return e.P.TrustInvest * trustStep * e.dealerOrdersToCome(d) * float64(e.avgListCents())
}

// trustAtStake is the worth (cents) of the trust that one late order would destroy: the
// dealer's count halves, and until on-time orders win it back, each order to come sells
// for 1% less per lost point.
func (e *Engine) trustAtStake(d *dealerBook) float64 {
	if e.Level < levelTrust || d.trust < 2 {
		return 0
	}
	lost := float64(d.trust - d.trust/2)
	toWin := e.dealerOrdersToCome(d)
	// Each won order wins one point back, so the points missed add up like a triangle.
	orders := math.Min(toWin, lost)
	return trustStep * (lost*orders - orders*(orders-1)/2) * float64(e.avgListCents())
}

// dealerOrdersToCome estimates the orders still to win from a dealer this week, from its pace so far.
func (e *Engine) dealerOrdersToCome(d *dealerBook) float64 {
	left := requestTimeLeft(e.now)
	if left <= 0 || left >= 1 {
		return 0
	}
	return float64(d.requests) * left / (1 - left) * e.P.TrustWinShare
}

// avgListCents is the mean list price of the requests seen so far.
func (e *Engine) avgListCents() int {
	if e.listCount == 0 {
		return 0
	}
	return e.listSum / e.listCount
}

// onQuoteAnswer learns from a won or lost quote.
func (e *Engine) onQuoteAnswer(requestID string, won bool) {
	q, ok := e.quotes[requestID]
	if !ok {
		return
	}
	delete(e.quotes, requestID)
	if b := e.market.beliefs; b != nil {
		e.market.count(e.market.dealer(q.Dealer), won)
		b.learn(q, won)
		return
	}
	surprise := e.market.learn(e.market.dealer(q.Dealer), q.Level, q.Ratio, won)
	e.market.learnToday(q.Dealer, q.Place, q.Day, surprise)
	e.market.learnDest(q.Dest, surprise)
}
