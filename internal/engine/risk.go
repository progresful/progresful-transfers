package engine

import (
	"math"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

const (
	wideRadiusKm     = 80.0
	tightSlackMin    = 300
	slackCapMin      = cal.Day
	thursdayIndex    = 3
	fridayIndex      = 4
	levelRiskTrained = 7 // the risk model was fitted on shortage worlds
	// Hours of the day the risk model's time features are measured against.
	riskHourSpan      = 10.0
	riskAfternoonHour = 14.0
	riskTodayLateHour = 13.0
)

// riskCoefficients are the weights of a logistic model of the chance that a won order
// arrives late. It uses only what is known when quoting: where drivers live, how much
// time there is, what the car needs, which delivery option it is, the weekday and hour,
// and how many of our own orders nearby still wait for a driver.
type riskCoefficients struct {
	bias, logPickup30, logDelivery30, logPickup80, logDelivery80 float64
	slackHours, tightSlack, pickupToday, premium, logExperienced float64
	privatePickup, privateDelivery, driveHours                   float64
	logBacklog, backlogPerDriver                                 float64
	thursday, friday, hourOfDay, afternoon                       float64
	sameDay, threeDays, todayAfternoon                           float64
}

// Two fitted models, chosen by Params.RiskModel. Both were fitted on practice weeks at level 8.
var (
	// riskModelSupply (RiskModel 2): 5,979 orders of early weeks played without any
	// filter, so it knows the whole range of requests. Used with the Thursday and
	// Friday factors on the cap. The champion's model.
	riskModelSupply = riskCoefficients{
		bias: 0.4468, logPickup30: 0.0665, logDelivery30: -0.0407, logPickup80: -0.6079, logDelivery80: -0.1467,
		slackHours: -0.0313, tightSlack: 1.0660, pickupToday: 1.3064, premium: 2.3522, logExperienced: -0.1410,
		privatePickup: -0.7232, privateDelivery: 0.1627, driveHours: -0.0427, friday: 0.0612,
		logBacklog: 0.3254, backlogPerDriver: -0.9766,
	}
	// riskModelTimed (RiskModel 3): 24,752 orders, adds weekday, hour and delivery option.
	// It ranks risk better (AUC 0.74 against 0.69 on held-out weeks) and gives more
	// on-time share at the same volume, but earned about 2.6% less margin on the bench.
	riskModelTimed = riskCoefficients{
		bias: -1.0670, logPickup30: 0.0834, logDelivery30: 0.0014, logPickup80: -0.5650, logDelivery80: -0.1011,
		slackHours: -0.0332, tightSlack: 0.7673, pickupToday: 1.1502, premium: 2.1715, logExperienced: -0.0876,
		privatePickup: -0.7215, privateDelivery: 0.6987, driveHours: 0.1398,
		logBacklog: 0.2248, backlogPerDriver: -0.2075,
		thursday: 0.4419, friday: 0.2589, hourOfDay: 0.0855, afternoon: 0.0467,
		sameDay: 0.4565, threeDays: -1.6553, todayAfternoon: 0.2747,
	}
)

const (
	// riskModelTimedID selects the model with weekday, hour and option features.
	riskModelTimedID = 3
	// riskModelLaneID selects the model that also knows the lane, the lead and the day's
	// load (risklane.go).
	riskModelLaneID = 4
)

func (e *Engine) riskModel() *riskCoefficients {
	if e.P.RiskModel == riskModelTimedID {
		return &riskModelTimed
	}
	return &riskModelSupply
}

// backlog counts our own orders that still wait for a driver and are collected within
// 80 km of a place: the cars that compete for the same drivers.
func (e *Engine) backlog(place string) int {
	n := 0
	for _, ob := range e.orders {
		if ob.cancelled || ob.done() || ob.bookings > 0 || len(ob.order.Legs) == 0 {
			continue
		}
		if e.Map.AirKm(place, ob.order.Legs[0].Pickup.PlaceID) <= wideRadiusKm {
			n++
		}
	}
	return n
}

// placesAround lists the places within 80 km of a place, itself included: one region
// for the competitor's outage.
func (e *Engine) placesAround(place string) []string {
	if near, ok := e.regions[place]; ok {
		return near
	}
	var near []string
	for _, p := range e.Map.Places() {
		if e.Map.AirKm(place, p.ID) <= wideRadiusKm {
			near = append(near, p.ID)
		}
	}
	e.regions[place] = near
	return near
}

// driversWithin counts the drivers living within km (straight line) of a place.
func (e *Engine) driversWithin(place string, km float64, experienced bool) int {
	key := place
	if km != notifyRadiusKm {
		key += "|wide"
	}
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
		if e.Map.AirKm(d.profile.HomePlaceID, place) <= km {
			n++
		}
	}
	e.reach[key] = n
	return n
}

func flag(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// legRisk is the model's chance that one car, won at minute at on the given delivery
// option with the given deadline, arrives late.
func (e *Engine) legRisk(leg *api.Leg, at, deadline int, option string) float64 {
	m := e.riskModel()
	pickup, delivery := leg.Pickup.PlaceID, leg.Delivery.PlaceID
	slack := e.latestPickup(leg, deadline) - max(at, leg.Pickup.Window.From)
	day, hour := cal.DayOf(at), float64(cal.MinuteOfDay(at))/cal.Hour
	today := cal.DayOf(leg.Pickup.Window.From) <= day
	waiting := float64(e.backlog(pickup))
	z := m.bias +
		m.logPickup30*math.Log1p(float64(e.driversWithin(pickup, notifyRadiusKm, false))) +
		m.logDelivery30*math.Log1p(float64(e.driversWithin(delivery, notifyRadiusKm, false))) +
		m.logPickup80*math.Log1p(float64(e.driversWithin(pickup, wideRadiusKm, false))) +
		m.logDelivery80*math.Log1p(float64(e.driversWithin(delivery, wideRadiusKm, false))) +
		m.slackHours*float64(min(max(slack, 0), slackCapMin))/cal.Hour +
		m.tightSlack*flag(slack < tightSlackMin) +
		m.pickupToday*flag(today) +
		m.privatePickup*flag(leg.Pickup.PrivateAddress) +
		m.privateDelivery*flag(leg.Delivery.PrivateAddress) +
		m.driveHours*float64(e.Map.Minutes(pickup, delivery))/cal.Hour +
		m.logBacklog*math.Log1p(waiting) +
		m.backlogPerDriver*waiting/math.Max(1, float64(e.driversWithin(pickup, wideRadiusKm, false))) +
		m.thursday*flag(day == thursdayIndex) +
		m.friday*flag(day == fridayIndex) +
		m.hourOfDay*(hour-requestDayStart/cal.Hour)/riskHourSpan +
		m.afternoon*flag(hour >= riskAfternoonHour) +
		m.sameDay*flag(option == api.SameDay) +
		m.threeDays*flag(option == api.Within3Days) +
		m.todayAfternoon*flag(today && hour >= riskTodayLateHour)
	if leg.Car.Premium {
		experienced := max(e.driversWithin(pickup, notifyRadiusKm, true), e.driversWithin(delivery, notifyRadiusKm, true))
		z += m.premium + m.logExperienced*math.Log1p(float64(experienced))
	}
	return sigmoid(z)
}

// lateRisk is the chance that an order won from the request on this option arrives
// late: the risk of its riskiest car. Zero below the levels the model was fitted on.
func (e *Engine) lateRisk(req *api.PriceRequest, opt api.DeliveryOption) float64 {
	if e.Level < levelRiskTrained {
		return 0
	}
	worst := 0.0
	ready, deadline := e.effective(req, opt)
	for i := range req.Legs {
		if e.sameVisit(req.Legs, i) {
			continue
		}
		leg, from, by := &req.Legs[i], req.RespondBy, opt.Deadline
		if i == 0 {
			leg, from, by = e.realLeg(req), ready, deadline
		}
		if e.P.RiskModel == riskModelLaneID {
			risk := e.laneRisk(leg, req.RespondBy, from, by, opt.Kind, e.riskRef(req), req.Options[0].ListPriceCents)
			worst = math.Max(worst, e.snowRisk(e.thinRisk(risk), leg, req.RespondBy, by))
			continue
		}
		worst = math.Max(worst, e.legRisk(leg, from, by, opt.Kind))
	}
	return worst
}

const (
	// guardPriorOrders and guardPriorLate steady the on-time outlook while few orders are in.
	guardPriorOrders = 30.0
	guardPriorLate   = 1.5
	// bookedLateRisk is the share of orders with a driver that still arrive late (drops, slow drives).
	bookedLateRisk = 0.015
	// With GuardGraceMin the prior is 30 orders with one late (96.7%, what a week ends at),
	// and a fresh order counts at least guardGraceRisk.
	guardGracePriorLate = 1.0
	guardGraceRisk      = 0.03
)

// unbookedLateRisk is the chance that an order without a driver ends late, by the minutes
// left to its last on-time pickup. Measured on practice weeks: 28% with 12 hours left,
// 36% with 6, 53% with 3, 90% with 1.
func unbookedLateRisk(minutesLeft int) float64 {
	switch {
	case minutesLeft <= 0:
		return 1
	case minutesLeft <= cal.Hour:
		return 0.9
	case minutesLeft <= 3*cal.Hour:
		return 0.6
	case minutesLeft <= 6*cal.Hour:
		return 0.42
	case minutesLeft <= 12*cal.Hour:
		return 0.32
	case minutesLeft <= cal.Day:
		return 0.22
	}
	return 0.14
}

// onTimeOutlook is the share of the week's orders expected to be on time: the orders
// already refunded are late, and each order still open counts by the chance it ends late
// from where it stands now (with or without a driver, and how much time is left). It
// looks ahead: late orders show in the account only hours or days after they were won.
func (e *Engine) onTimeOutlook() float64 {
	orders, late := guardPriorOrders, guardPriorLate
	if e.P.GuardGraceMin > 0 {
		late = guardGracePriorLate
	}
	for _, ob := range e.orders {
		if ob.cancelled {
			continue
		}
		orders++
		switch {
		case ob.refunded:
			late++
		case ob.paid || ob.done():
		default:
			late += e.openLateRisk(ob)
		}
	}
	return 1 - late/orders
}

// openLateRisk is the chance that an undelivered order ends late, from its state now.
func (e *Engine) openLateRisk(ob *orderBook) float64 {
	risk := 0.0
	for i := range ob.legs {
		lb := &ob.legs[i]
		switch {
		case lb.delivered:
		case lb.pickedUp:
			risk = math.Max(risk, bookedLateRisk)
		case lb.gig != nil && (lb.gig.status == gigBooked || lb.gig.status == gigWorking):
			risk = math.Max(risk, bookedLateRisk)
		default:
			waiting := unbookedLateRisk(e.lastPickupOf(ob, []int{i}) - e.now)
			if e.P.GuardGraceMin > 0 && ob.firstBookedAt == 0 && e.now-ob.wonAt < e.P.GuardGraceMin {
				// Just won: it has not had the time to find a driver, and counts by the
				// risk it was quoted on, not as an order that nobody wants.
				waiting = math.Min(waiting, math.Max(ob.quotedRisk, guardGraceRisk))
			}
			risk = math.Max(risk, waiting)
		}
	}
	return risk
}

// thinRisk is the modelled lateness risk of a request, raised by ThinRisk in a world with
// fewer than ThinRoster drivers. The lane model counts the drivers near a pickup, but a
// world short of drivers runs late everywhere: its weeks were late 1.3-1.4 times as often
// as modelled, the weeks with more drivers 0.7-1.0 times.
func (e *Engine) thinRisk(risk float64) float64 {
	if e.P.ThinRisk <= 0 || len(e.drivers) == 0 || len(e.drivers) >= e.P.ThinRoster {
		return risk
	}
	return math.Min(1, risk*e.P.ThinRisk)
}

// riskCap is the highest modelled lateness risk still quoted to a dealer: RiskMax,
// tightened for a trusted dealer and by the guard as the week's on-time share nears the
// delivery minimum.
func (e *Engine) riskCap(d *dealerBook) float64 {
	limit := e.P.RiskMax
	// Orders won late in the week arrive late more often (weekend ahead, drivers who have
	// learned to wait, no later days to make up for them): 8.5% on Thursdays and 11.6% on
	// Fridays against 4% early in the week.
	switch cal.DayOf(e.now) {
	case 0:
		if e.P.RiskMonday > 0 {
			limit *= e.P.RiskMonday
		}
	case thursdayIndex:
		limit *= e.P.RiskThursday
	case fridayIndex:
		limit *= e.P.RiskFriday
	}
	if e.Level >= levelTrust && d != nil {
		// A late order halves a dealer's trust: the more there is to lose, the less risk.
		limit *= 1 - e.P.TrustRiskCut*float64(d.trust)/maxTrust
	}
	limit *= e.stuckFactor()
	if e.P.GuardFull <= e.P.GuardFloor || e.P.GuardFull <= 0 {
		return limit
	}
	outlook := e.onTimeOutlook()
	room := (outlook - e.P.GuardFloor) / (e.P.GuardFull - e.P.GuardFloor)
	return limit * math.Max(e.P.GuardMin, math.Min(1, room)) * (1 + (e.P.LoosenRisk-1)*e.spare(outlook))
}

const (
	// stuckAfterMin is how long after it was won an order counts for the stuck guard, and
	// stuckMinOrders how many such orders the guard needs before it says anything.
	stuckAfterMin  = 3 * cal.Hour
	stuckMinOrders = 30
)

// stuckShare is the share of the orders won at least stuckAfterMin ago that have never
// had a driver: no booking so far, no car collected.
func (e *Engine) stuckShare() float64 {
	old, stuck := 0, 0
	for _, ob := range e.orders {
		if ob.cancelled || e.now-ob.wonAt < stuckAfterMin {
			continue
		}
		old++
		if ob.firstBookedAt == 0 && !ob.done() && e.neverCarried(ob) {
			stuck++
		}
	}
	if old < stuckMinOrders {
		return 0
	}
	return float64(stuck) / float64(old)
}

// neverCarried reports whether no car of the order has a driver or has been moved. After
// a restart the time of an order's first booking is not known, its gigs are.
func (e *Engine) neverCarried(ob *orderBook) bool {
	for i := range ob.legs {
		lb := &ob.legs[i]
		if lb.delivered || lb.pickedUp || (lb.gig != nil && (lb.gig.status == gigBooked || lb.gig.status == gigWorking)) {
			return false
		}
	}
	return true
}

// stuckFactor is the stuck guard's factor on the risk cap: 1 up to StuckFrom, StuckMin
// from StuckFull.
func (e *Engine) stuckFactor() float64 {
	if e.P.StuckFrom <= 0 || e.P.StuckFull <= e.P.StuckFrom {
		return 1
	}
	over := (e.stuckShare() - e.P.StuckFrom) / (e.P.StuckFull - e.P.StuckFrom)
	return math.Max(e.P.StuckMin, math.Min(1, 1-over*(1-e.P.StuckMin)))
}

// spare is how far the week's on-time outlook is above what is needed, from 0 (at
// LoosenFrom or under) to 1 (at LoosenFull or over): a sign of drivers to spare.
func (e *Engine) spare(outlook float64) float64 {
	if e.P.LoosenFull <= e.P.LoosenFrom {
		return 0
	}
	return math.Max(0, math.Min(1, (outlook-e.P.LoosenFrom)/(e.P.LoosenFull-e.P.LoosenFrom)))
}
