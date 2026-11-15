package engine

import (
	"math"

	"progresful-transfers/internal/api"
)

// Quoting again. A request is answered the moment it arrives, and the dealer decides 15
// minutes later on the last quote it holds. In those minutes other dealers answer other
// quotes, and each answer moves what the engine believes about the market: most at the
// start of a day, inside a batch that arrives over half an hour, and on the morning of a
// price war or an outage. A few minutes before a request's deadline the engine prices it
// once more, and sends the new prices when they differ enough from the ones it sent.

// nextRequoteWake is the next minute at which a pending quote is due to be looked at again.
func (e *Engine) nextRequoteWake() int {
	wake := math.MaxInt32
	if e.P.RequoteLead <= 0 {
		return wake
	}
	for _, q := range e.quotes {
		if q.Again || len(q.Prices) == 0 {
			continue
		}
		if at := q.By - e.P.RequoteLead; at > e.now {
			wake = min(wake, at)
		}
	}
	return wake
}

// requoteRound prices again the pending quotes whose deadline is near, in the order of
// their requests, so a run is the same every time.
func (e *Engine) requoteRound() {
	if e.P.RequoteLead <= 0 || e.replay {
		return
	}
	var due []string
	for id, q := range e.quotes {
		if !q.Again && len(q.Prices) > 0 && e.now >= q.By-e.P.RequoteLead && e.now < q.By {
			due = append(due, id)
		}
	}
	sortStrings(due)
	for _, id := range due {
		e.requote(id)
	}
}

// requote prices one pending request again and sends the new quote when its first
// option's price has moved by RequoteMin or more. Either way the request is not looked at
// a third time.
func (e *Engine) requote(id string) {
	old := e.quotes[id]
	old.Again = true
	e.quotes[id] = old
	req := e.reqs[id]
	if req == nil || len(req.Options) == 0 {
		return
	}
	stats := e.stats.snapshotDeclines()
	why, risk, limit, down, level, war := e.why, e.whyRisk, e.whyCap, e.whyDown, e.whyLvl, e.whyWar
	prices := e.pricesFor(req) // replaces the pending quote when it prices the request
	e.stats.restoreDeclines(stats)
	e.why, e.whyRisk, e.whyCap, e.whyDown, e.whyLvl, e.whyWar = why, risk, limit, down, level, war
	first := req.Options[0].Kind
	was, now := old.Prices[first], prices[first]
	if was <= 0 || now <= 0 || math.Abs(float64(now-was)) < e.P.RequoteMin*float64(was) {
		e.quotes[id] = old // the quote the dealer holds stands
		return
	}
	q := e.quotes[id]
	q.Prices, q.By, q.Again = prices, old.By, true
	e.quotes[id] = q
	e.stats.Requotes++
	e.send(api.Action{Type: api.ActQuote, RequestID: id, Prices: prices}, nil)
}
