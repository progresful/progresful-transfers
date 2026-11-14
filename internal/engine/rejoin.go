package engine

import (
	"encoding/json"
	"fmt"
	"os"

	"highwaycoins/internal/api"
)

// Snapshot gig statuses that still carry cars.
const (
	snapOpen    = "open"
	snapBooked  = "booked"
	snapWorking = "working"
	orderCancel = "cancelled"
)

// Rejoin rebuilds the books from the world after a start in the middle of a week (the
// final, or a restart). It replays the history without acting and takes the orders and
// live gigs from the snapshot. Resume then acts on what is open.
func (e *Engine) Rejoin(snap *api.Snapshot, roster []api.DriverProfile, history []api.Event) {
	e.Begin(snap, roster)
	e.now = 0 // the replay walks the clock through the history
	e.replay = true
	for i := range history {
		e.now = max(e.now, history[i].At)
		e.handle(&history[i])
	}
	e.replay = false
	e.outbox = nil
	e.now = snap.Now
	for i := range snap.Orders {
		e.adoptOrder(&snap.Orders[i])
	}
	for i := range snap.Gigs {
		e.adoptGig(&snap.Gigs[i])
	}
}

// Resume answers what is open after a Rejoin (and after LoadState, so that quotes use the
// saved market): requests that can still be quoted, and cars without a gig. Open requests
// are quoted again: the replay cannot tell whether the earlier process reached them, and
// a new quote simply replaces an old one.
func (e *Engine) Resume(snap *api.Snapshot) {
	for i := range snap.OpenRequests {
		e.onRequest(&snap.OpenRequests[i])
	}
	e.adoptOrphans()
}

// adoptOrder takes the world's current view of an order.
func (e *Engine) adoptOrder(o *api.Order) {
	ob := e.orders[o.ID]
	if ob == nil {
		e.replay = true
		e.onWon(o, e.now)
		e.replay = false
		ob = e.orders[o.ID]
	}
	if ob == nil {
		return
	}
	ob.order = *o
	e.applyFacts(ob)
	ob.cancelled = o.Status == orderCancel
	for i, d := range o.DeliveredAt {
		if i < len(ob.legs) {
			ob.legs[i].delivered = d != nil
			ob.legs[i].gig = nil
		}
	}
}

// adoptGig takes a live gig from the snapshot into the books.
func (e *Engine) adoptGig(sg *api.Gig) {
	if sg.Status != snapOpen && sg.Status != snapBooked && sg.Status != snapWorking {
		return
	}
	g := &gigBook{id: sg.ID, plan: sg.Schedule, shown: sg.FeeCents, postedAt: sg.PostedAt, mode: modeLate, fee: sg.FeeCents}
	switch sg.Status {
	case snapOpen:
		g.status = gigOpen
	case snapBooked:
		g.status = gigBooked
	default:
		g.status = gigWorking
	}
	if sg.DriverID != nil {
		g.driver = *sg.DriverID
	}
	audience := sg.Audience
	g.post = api.Action{Type: api.ActPost, Stops: sg.Stops, Audience: &audience, Schedule: sg.Schedule}
	for _, stop := range sg.Stops {
		ob := e.orders[stop.OrderID]
		if ob == nil || stop.Leg >= len(ob.legs) || ob.legs[stop.Leg].delivered {
			continue
		}
		if stop.Kind == stopDelivery && stop.Window != nil {
			// Measured from the deadline that binds (the note's, if earlier), so a tight
			// gig with a note deadline reads as buffer 0, not as a long buffer.
			g.mode = modeBuffered
			g.buffer = max(0, ob.deadlineOf()-stop.Window.To)
		}
		if stop.Kind != stopPickup {
			continue
		}
		if stop.Window != nil && ob.narrowed {
			g.mode, g.bookableUntil = modeDay, stop.Window.To
			g.dayLate = stop.Window.From > ob.deadlineOf()
		}
		g.legs = append(g.legs, legRef{orderID: stop.OrderID, leg: stop.Leg})
		ob.legs[stop.Leg].gig = g
		if g.status != gigOpen && ob.bookings == 0 {
			ob.bookings = 1 // a driver holds it: it does not wait for one
		}
		g.base += e.baseFee(ob, []int{stop.Leg})
	}
	if len(g.legs) == 0 {
		return
	}
	if len(g.plan) == 0 {
		g.plan = []api.FeeStep{{At: e.now, FeeCents: max(minFeeCents, sg.FeeCents)}}
	}
	if e.P.Chains == 1 && g.status == gigOpen && e.isChain(g) {
		// Only an open chain is still an attempt; one a driver holds is just a booked gig.
		e.adoptChain(g)
	}
	switch last := e.lastPickupOfGig(g); {
	case g.mode == modeDay, g.mode == modeChain:
	case g.mode == modeBuffered:
		g.bookableUntil = last - g.buffer
	case last > e.now:
		g.mode, g.bookableUntil = modeTight, last
	}
	e.gigs[g.id] = g
}

// sortedOrders lists the orders by id, so the engine acts in the same order every time.
func (e *Engine) sortedOrders() []*orderBook {
	ids := make([]string, 0, len(e.orders))
	for id := range e.orders {
		ids = append(ids, id)
	}
	sortStrings(ids)
	out := make([]*orderBook, len(ids))
	for i, id := range ids {
		out[i] = e.orders[id]
	}
	return out
}

// savedState is what cannot be rebuilt from the world's events: what the engine learned
// about prices, and the quotes still waiting for an answer.
type savedState struct {
	Mu      float64                 `json:"mu"`
	Seen    int                     `json:"seen"`
	Dealers map[string]savedDealer  `json:"dealers"`
	Quotes  map[string]pendingQuote `json:"quotes"`
	Dest    map[string]savedDest    `json:"dest,omitempty"`
}

type savedDest struct {
	Offset float64 `json:"offset"`
	Quotes int     `json:"quotes"`
}

type savedDealer struct {
	Offset float64 `json:"offset"`
	Quotes int     `json:"quotes"`
	Wins   int     `json:"wins"`
}

// SaveState writes the learned market to a file, atomically.
func (e *Engine) SaveState(path string) error {
	state := savedState{Mu: e.market.mu, Seen: e.market.seen, Dealers: map[string]savedDealer{}, Quotes: e.quotes}
	for id, d := range e.market.dealers {
		state.Dealers[id] = savedDealer{Offset: d.offset, Quotes: d.quotes, Wins: d.wins}
	}
	if len(e.market.dest) > 0 {
		state.Dest = map[string]savedDest{}
		for place, b := range e.market.dest {
			state.Dest[place] = savedDest{Offset: b.offset, Quotes: b.quotes}
		}
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode engine state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write engine state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace engine state: %w", err)
	}
	return nil
}

// LoadState restores the learned market saved by an earlier process. A missing file is not an error.
func (e *Engine) LoadState(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read engine state: %w", err)
	}
	var state savedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode engine state: %w", err)
	}
	e.market.mu, e.market.seen = state.Mu, state.Seen
	for id, sd := range state.Dealers {
		d := e.market.dealer(id)
		d.offset, d.quotes, d.wins = sd.Offset, sd.Quotes, sd.Wins
	}
	for id, q := range state.Quotes {
		e.quotes[id] = q
	}
	for place, sd := range state.Dest {
		e.market.dest[place] = &destBook{offset: sd.Offset, quotes: sd.Quotes}
	}
	return nil
}
