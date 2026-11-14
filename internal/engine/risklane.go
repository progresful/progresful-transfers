package engine

import (
	"math"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// The lane risk model (Params.RiskModel 4): the chance that a won order arrives late,
// from what is known when it is quoted.
//
// It adds to the earlier models what decides most late orders in a shortage:
//   - the lane and the lead. A long drive with little time to find a driver is the
//     strongest sign: hauls over 500 km won 12 to 30 hours before their last on-time
//     pickup are late 21% of the time, the same hauls with 30 hours or more 7%, and
//     lanes of 30 to 150 km about 1%. Hops under 30 km are late 4%: their fee is too
//     small to bring a driver from another town.
//   - same-day orders. A car that is ready today and due today has only today's drivers:
//     late 17% on a Friday, 15% when won after 14:00, and 27% once sixty orders have
//     been won that day, against 3.5% when won before 10:00.
//   - the day's load: orders won today, orders still without a driver, orders won in the
//     last two hours.
//
// Unlike the earlier models it measures the weekday, the hour and the lead from the
// minute the dealer answers, not from the minute the car is ready, and "ready today"
// means ready on the day of the answer. Its output is a calibrated chance (4% on
// average), so RiskMax is lower with it than with the supply model.

const (
	laneFeatureCount = 38
	// Bounds of the features, as in the fit.
	riskShortKm      = 30.0
	riskLeadCapMin   = 3 * cal.Day
	riskMinDriveMin  = 30
	riskLeadPerDrive = 20.0
	riskMinFeeCents  = 100
	riskFeeUnit      = 100.0
	riskRecentMin    = 2 * cal.Hour
)

// load is how busy the engine's own book is at the moment of a quote.
type load struct {
	wonToday int // orders won earlier today
	open     int // orders that never had a driver yet
	recent   int // orders won in the last two hours
}

// bookLoad counts the orders that make up the day's load.
func (e *Engine) bookLoad() load {
	var l load
	today := cal.DayOf(e.now)
	for _, ob := range e.orders {
		if ob.cancelled {
			continue
		}
		if cal.DayOf(ob.wonAt) == today {
			l.wonToday++
		}
		if ob.wonAt >= e.now-riskRecentMin {
			l.recent++
		}
		if !ob.done() && ob.bookings == 0 {
			l.open++
		}
	}
	return l
}

// laneFeatures are the model's inputs for one car, in the order of riskLaneWeights. at
// is the minute the dealer answers, ready the first minute the car can be collected,
// fee and list the request's typical fee and the list price of its first option.
func (e *Engine) laneFeatures(leg *api.Leg, at, ready, deadline int, option string, fee, list int) [laneFeatureCount]float64 {
	pickup, delivery := leg.Pickup.PlaceID, leg.Delivery.PlaceID
	last := e.latestPickup(leg, deadline)
	slack := last - ready
	lead := max(last-at, 0)
	drive := e.Map.Minutes(pickup, delivery)
	km := e.Map.Km(pickup, delivery)
	day, hour := cal.DayOf(at), float64(cal.MinuteOfDay(at))/cal.Hour
	today := flag(cal.DayOf(leg.Pickup.Window.From) <= day)
	sameDay := flag(option == api.SameDay)
	waiting := float64(e.backlog(pickup))
	l := e.bookLoad()
	driveHours := float64(drive) / cal.Hour
	hourOfDay := (hour - requestDayStart/cal.Hour) / riskHourSpan
	logLead := math.Log1p(float64(lead) / cal.Hour)
	leadPerDrive := math.Min(riskLeadPerDrive, float64(lead)/float64(max(riskMinDriveMin, drive)))
	logWonToday, logOpen := math.Log1p(float64(l.wonToday)), math.Log1p(float64(l.open))
	logFee, feeOverList := 0.0, 0.0
	if fee > 0 && list > 0 {
		logFee = math.Log(float64(max(riskMinFeeCents, fee)) / riskFeeUnit)
		feeOverList = math.Log(float64(fee) / float64(list))
	}
	todaySame := today * sameDay
	return [laneFeatureCount]float64{
		math.Log1p(float64(e.driversWithin(pickup, notifyRadiusKm, false))),
		math.Log1p(float64(e.driversWithin(delivery, notifyRadiusKm, false))),
		math.Log1p(float64(e.driversWithin(pickup, wideRadiusKm, false))),
		math.Log1p(float64(e.driversWithin(delivery, wideRadiusKm, false))),
		float64(min(max(slack, 0), slackCapMin)) / cal.Hour,
		flag(slack < tightSlackMin),
		today,
		flag(leg.Pickup.PrivateAddress),
		flag(leg.Delivery.PrivateAddress),
		driveHours,
		math.Log1p(waiting),
		waiting / math.Max(1, float64(e.driversWithin(pickup, wideRadiusKm, false))),
		flag(day == thursdayIndex),
		flag(day == fridayIndex),
		hourOfDay,
		flag(hour >= riskAfternoonHour),
		sameDay,
		flag(option == api.Within3Days),
		today * flag(hour >= riskTodayLateHour),
		logFee,
		feeOverList,
		flag(km < riskShortKm),
		flag(km >= longLaneKm),
		float64(min(lead, riskLeadCapMin)) / cal.Hour,
		logLead,
		leadPerDrive,
		logWonToday,
		logOpen,
		math.Log1p(float64(l.recent)),
		todaySame,
		todaySame * flag(day == fridayIndex),
		todaySame * flag(day == thursdayIndex),
		todaySame * hourOfDay,
		todaySame * leadPerDrive,
		todaySame * logLead,
		todaySame * driveHours,
		todaySame * logWonToday,
		todaySame * logOpen,
	}
}

// laneRisk is the lane model's chance that one car arrives late.
func (e *Engine) laneRisk(leg *api.Leg, at, ready, deadline int, option string, fee, list int) float64 {
	z := riskLaneBias
	for i, x := range e.laneFeatures(leg, at, ready, deadline, option, fee, list) {
		z += riskLaneWeights[i] * x
	}
	if leg.Car.Premium {
		// Premium cars were declined in the weeks of the fit: the supply model's terms stand in.
		experienced := max(e.driversWithin(leg.Pickup.PlaceID, notifyRadiusKm, true), e.driversWithin(leg.Delivery.PlaceID, notifyRadiusKm, true))
		z += riskModelSupply.premium + riskModelSupply.logExperienced*math.Log1p(float64(experienced))
	}
	return sigmoid(z)
}
