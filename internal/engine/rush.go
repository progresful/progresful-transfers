package engine

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// The trust rush (level 8). A dealer counts the orders delivered to it on time, up to 25,
// and accepts prices 1% higher for each. The count moves at the delivery, and a car won
// on Monday with a deadline on Tuesday is delivered on Tuesday: on Monday evening the
// three biggest dealers, who send half of all requests, stand at 2.6 on average, on
// Tuesday at 12.8. A car of such a dealer that is ready today is therefore posted to
// arrive today. If no driver takes it while that is still possible, it is posted again
// with its own deadline and has lost nothing but the hours.

const (
	// rushDayEnd is the minute of the day by which a rushed car must arrive: the hour of every deadline.
	rushDayEnd = 20 * cal.Hour
	// rushMinRequests is how many requests must be in before a dealer's share of them counts.
	rushMinRequests = 20
	// rushBufferMin tells a rush gig from a buffered one when a world is joined again: its
	// delivery window ends at least this long before the order's deadline.
	rushBufferMin = 12 * cal.Hour
)

// rushable reports whether the order's one free car would be posted for today.
func (e *Engine) rushable(ob *orderBook, legs []int) bool {
	if e.P.TrustRush <= 0 || len(legs) != 1 {
		return false
	}
	leg := &ob.order.Legs[legs[0]]
	first := max(e.now, earliestPickup(leg, max(e.now, ob.legs[legs[0]].notBefore)))
	w, _ := e.rushWindow(ob, legs, first, e.buffer(ob, legs))
	return w != nil
}

// rushWindow is the delivery window of a car that is to arrive today, and the last minute
// a driver can collect it for that; nil when the order is not one to rush. first is the
// first minute the car can be collected, buffer the room kept before the site closes.
func (e *Engine) rushWindow(ob *orderBook, legs []int, first, buffer int) (*api.TimeWindow, int) {
	if e.P.TrustRush <= 0 || e.Level < levelTrust || ob.rushed || len(legs) != 1 || len(ob.order.Legs) != 1 {
		return nil, 0
	}
	day := cal.DayOf(e.now)
	if day >= requestDays-1 {
		return nil, 0 // Friday or later: no request is left to sell the trust into
	}
	leg := &ob.order.Legs[legs[0]]
	if leg.Pickup.PrivateAddress || leg.Delivery.PrivateAddress || ob.narrowed || ob.noteDeadline > 0 || ob.legs[legs[0]].plain {
		return nil, 0
	}
	d := e.market.dealer(ob.order.DealerID)
	if d.trust >= maxTrust || e.stats.Requests < rushMinRequests || float64(d.requests) < e.P.TrustRush*float64(e.stats.Requests) {
		return nil, 0
	}
	today := cal.At(day, rushDayEnd)
	if ob.deadlineOf() <= today {
		return nil, 0 // due today anyway
	}
	arrive := cal.LastOpen(leg.Delivery.Hours, today)
	if arrive < 0 || cal.DayOf(arrive) != day {
		return nil, 0
	}
	to := arrive - buffer
	until := to - e.driveMin(leg)
	if until-first < e.P.TrustRushMin || to <= leg.Delivery.Window.From {
		return nil, 0
	}
	return &api.TimeWindow{From: leg.Delivery.Window.From, To: to}, until
}
