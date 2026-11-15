package engine

import (
	"math"
	"sort"

	"progresful-transfers/internal/api"
)

// The typical fee of a request is the lane's fee, times one factor per world, plus a noise
// of its own (13% s.d.). Drivers know nothing of that noise: what a gig costs them is the
// lane. The fee clock is anchored on the typical fee, so a request whose noise is upward
// opens above what the lane's drivers need and is taken at once, dearer: the orders whose
// typical fee was 12-16% over the lane's paid 1.00 times the lane's fee, those at the
// lane's level 0.91, and no more of them were booked at any given fee.
//
// With RefCap the engine learns the world's factor from the requests it has seen (the
// median of the typical fee over the lane's table) and caps the fee base of an order at
// RefCap times the lane's fee. A typical fee under the lane's level is left alone.

// refCapMinSeen is how many requests must be in before the world's factor is trusted.
const refCapMinSeen = 20

// noteRef takes a new request into the world's factor and, with RefCap, records the
// capped typical fee of a request whose own is above the lane's.
func (e *Engine) noteRef(req *api.PriceRequest) {
	if e.P.RefCap <= 0 || e.Level < levelTrust || len(req.Legs) != 1 || req.ReferenceFeeCents <= 0 {
		return
	}
	lane, ok := laneLogRef[req.Legs[0].Pickup.PlaceID+"|"+req.Legs[0].Delivery.PlaceID]
	if !ok {
		return
	}
	resid := math.Log(float64(req.ReferenceFeeCents)) - lane
	if len(e.refResid) >= refCapMinSeen {
		offset := e.refResid[len(e.refResid)/2] // the median of the requests before this one
		e.laneFee[req.ID] = math.Exp(lane + offset)
		if e.cappedAt(req, e.P.RefCap) < req.ReferenceFeeCents {
			e.stats.RefCapped++
		}
	}
	at := sort.SearchFloat64s(e.refResid, resid)
	grown := make([]float64, 0, len(e.refResid)+1)
	grown = append(append(append(grown, e.refResid[:at]...), resid), e.refResid[at:]...)
	e.refResid = grown
}

// cappedAt is the request's typical fee, capped at the given multiple of its lane's fee
// when that is known.
func (e *Engine) cappedAt(req *api.PriceRequest, multiple float64) int {
	lane, ok := e.laneFee[req.ID]
	if !ok || multiple <= 0 {
		return req.ReferenceFeeCents
	}
	return min(req.ReferenceFeeCents, int(math.Round(lane*multiple)))
}

// fairRef is the typical fee that the fee clock is anchored on: the request's own, or the
// lane's cap when the request's is above it. The clock's cap is RefCapClock when that is
// set, else RefCap.
func (e *Engine) fairRef(req *api.PriceRequest) int {
	if e.P.RefCapClock > 0 {
		return e.cappedAt(req, e.P.RefCapClock)
	}
	return e.cappedAt(req, e.P.RefCap)
}

// riskRef is the typical fee that the lateness model reads: the capped one with
// RefCapRisk, else the request's own. The model learned that a high typical fee means a
// car that is booked sooner, which was the fee clock's doing; once the clock is anchored
// on the capped fee, the capped fee is what tells how soon.
func (e *Engine) riskRef(req *api.PriceRequest) int {
	if e.P.RefCapRisk == 1 {
		return e.fairRef(req)
	}
	return req.ReferenceFeeCents
}

// priceRef is the typical fee that a quote's cost is built on: the capped one with
// RefCapPrice, else the request's own.
func (e *Engine) priceRef(req *api.PriceRequest) int {
	if e.P.RefCapPrice == 1 {
		return e.cappedAt(req, e.P.RefCap)
	}
	return req.ReferenceFeeCents
}
