package engine

import (
	"math"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// feeModel predicts the fee at which a gig gets booked, as a multiple of its typical fee,
// from what is known when quoting. Fitted on 9,770 booked single-car orders of 29 level 8
// practice weeks (log-linear; R2 0.19 on held-out worlds: bookings are noisy, but the
// direction is clear). What a gig costs is decided by where it ends (drivers going home),
// when the car is ready and the weekday, not by who hears of it at the pickup.
var feeModel = struct {
	bias, readyToday, logDelivery30, logPickup30, dayIndex, friday float64
	longLane, shortLane, afternoon, sameDay, threeDays, private    float64
}{
	bias: -0.0642, readyToday: 0.1346, logDelivery30: -0.0258, logPickup30: -0.0045, dayIndex: 0.0124, friday: -0.0069,
	longLane: 0.0409, shortLane: 0.0908, afternoon: 0.0226, sameDay: 0.0181, threeDays: -0.0242, private: 0.1121,
}

const (
	feeModelAfternoon = 14 * cal.Hour
	feeModelLastDay   = 4
)

// expectedFeeMult is the model's booking fee of one car as a multiple of its typical fee.
func (e *Engine) expectedFeeMult(leg *api.Leg, at int, option string) float64 {
	m := &feeModel
	day := min(cal.DayOf(at), feeModelLastDay)
	km := e.Map.Km(leg.Pickup.PlaceID, leg.Delivery.PlaceID)
	z := m.bias +
		m.readyToday*flag(cal.DayOf(leg.Pickup.Window.From) <= cal.DayOf(at)) +
		m.logDelivery30*math.Log1p(float64(e.driversWithin(leg.Delivery.PlaceID, notifyRadiusKm, false))) +
		m.logPickup30*math.Log1p(float64(e.driversWithin(leg.Pickup.PlaceID, notifyRadiusKm, false))) +
		m.dayIndex*float64(day) +
		m.friday*flag(day == fridayIndex) +
		m.longLane*flag(km >= longLaneKm) +
		m.shortLane*flag(km < shortLaneKm) +
		m.afternoon*flag(cal.MinuteOfDay(at) >= feeModelAfternoon) +
		m.sameDay*flag(option == api.SameDay) +
		m.threeDays*flag(option == api.Within3Days) +
		m.private*flag(leg.Pickup.PrivateAddress || leg.Delivery.PrivateAddress)
	return math.Exp(z)
}

// gigFeeMult is the expected booking fee multiple of a gig posted now: that of its first car.
func (e *Engine) gigFeeMult(ob *orderBook, legs []int) float64 {
	if len(legs) == 0 {
		return 1
	}
	return e.expectedFeeMult(&ob.order.Legs[legs[0]], e.now, ob.order.Option)
}
