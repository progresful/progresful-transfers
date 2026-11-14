package engine

import (
	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// dealerCost is the factor on the cost of a request for who its dealer is.
//
// One dealer sends three requests in ten, three send half, and a dealer's trust (up to
// 25% on its price) grows only with the orders delivered to it on time. Two switches act
// on that: a lower cost for a big dealer's short cars that can still arrive today, while
// its trust can grow (LeadCost), and a higher cost for the many small dealers, whose
// orders sell at the lowest prices (MinorCost). Shares count once rushMinRequests
// requests are in.
func (e *Engine) dealerCost(req *api.PriceRequest, d *dealerBook) float64 {
	if e.Level < levelTrust || e.stats.Requests < rushMinRequests {
		return 1
	}
	share := float64(d.requests) / float64(e.stats.Requests)
	if e.P.LeadCost > 0 && e.P.LeadCost != 1 && share >= e.P.LeadShare && d.trust < maxTrust && e.arrivesToday(req) {
		return e.P.LeadCost
	}
	if e.P.MinorCost > 0 && e.P.MinorCost != 1 && share < e.P.MinorShare {
		return e.P.MinorCost
	}
	return 1
}

// arrivesToday reports whether a request's one car is short enough (LeadMaxDrive) and ready
// early enough to be collected and delivered today, with the room a driver needs.
func (e *Engine) arrivesToday(req *api.PriceRequest) bool {
	if len(req.Legs) != 1 {
		return false
	}
	leg := &req.Legs[0]
	if leg.Pickup.PrivateAddress || leg.Delivery.PrivateAddress {
		return false
	}
	day := cal.DayOf(e.now)
	if day >= requestDays-1 {
		return false // Friday: no later request to sell the trust into
	}
	drive := e.driveMin(leg)
	if drive > e.P.LeadMaxDrive {
		return false
	}
	arrive := cal.LastOpen(leg.Delivery.Hours, cal.At(day, rushDayEnd))
	if arrive < 0 || cal.DayOf(arrive) != day {
		return false
	}
	first := earliestPickup(leg, req.RespondBy)
	return arrive-drive-first >= e.P.TrustRushMin
}
