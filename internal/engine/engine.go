package engine

import (
	"sort"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/geo"
	"highwaycoins/internal/notes"
)

const (
	// levelSchedules is the first level with multi-step fee schedules.
	levelSchedules = 2
	// levelNetwork is the first level with multi-car gigs and direct offers.
	levelNetwork = 3
	// levelQuotes is the first level where the dealer compares prices.
	levelQuotes = 4
)

// Engine keeps the books of one world and decides what to do at each step.
// It is not safe for concurrent use: one goroutine drives one engine.
type Engine struct {
	P     Params
	Level int
	Map   *geo.Map
	// Logf receives one line per notable decision; nil keeps the engine quiet.
	Logf func(format string, args ...any)

	now     int
	endsAt  int
	drivers map[string]*driverBook
	reach   map[string]int      // place id -> drivers the app notifies of a pickup there
	regions map[string][]string // place id -> the places within 80 km of it
	reqs    map[string]*api.PriceRequest
	orders  map[string]*orderBook
	gigs    map[string]*gigBook
	outbox  []Pending
	stats   Stats
	// replay is set while Rejoin feeds old events through the books: nothing is sent.
	replay bool

	facts     map[string]notes.Facts // request id -> what its note means
	market    *market
	quotes    map[string]pendingQuote // request id -> what was quoted
	listSum   int                     // sum and count of first-option list prices seen
	listCount int
	winsDay   int // day index of winsToday
	winsToday int // orders won on winsDay
}

// New makes an engine for a world of the given level.
func New(level int, p Params, m *geo.Map) *Engine {
	e := &Engine{
		P:       p,
		Level:   level,
		Map:     m,
		drivers: map[string]*driverBook{},
		reach:   map[string]int{},
		regions: map[string][]string{},
		reqs:    map[string]*api.PriceRequest{},
		orders:  map[string]*orderBook{},
		gigs:    map[string]*gigBook{},
		stats:   newStats(),
		quotes:  map[string]pendingQuote{},
		facts:   map[string]notes.Facts{},
	}
	if e.P.MarketStart <= 0 {
		e.P.MarketStart = marketStartFor(level)
	}
	e.market = newMarket(e.P, level)
	e.market.around = e.placesAround
	return e
}

func (e *Engine) logf(format string, args ...any) {
	if e.Logf != nil {
		e.Logf(cal.Format(e.now)+" "+format, args...)
	}
}

// Begin sets the clock and the driver list at the start of a run.
func (e *Engine) Begin(snap *api.Snapshot, roster []api.DriverProfile) {
	e.now, e.endsAt = snap.Now, snap.EndsAt
	for _, d := range roster {
		e.driver(d.ID).profile = d
	}
}

// WakeKinds are the events that need an answer; a practice clock stops at them.
func (e *Engine) WakeKinds() []string {
	return []string{
		api.KindPriceRequest, api.KindOrderWon, api.KindOrderChanged, api.KindOrderCancelled,
		api.KindVisitFailed, api.KindGigExpired, api.KindDeadlineClose, api.KindGigDropped,
		api.KindNoShow, api.KindDriverLate, api.KindOfferAnswered,
	}
}

// Now is the engine's clock.
func (e *Engine) Now() int { return e.now }

// SetNow moves the engine's clock; it never goes back.
func (e *Engine) SetNow(now int) { e.now = max(e.now, now) }

func (e *Engine) send(a api.Action, done func(api.ActionResult)) {
	if e.replay {
		return
	}
	e.outbox = append(e.outbox, Pending{Action: a, done: done})
}

func sortStrings(s []string) { sort.Strings(s) }

// Drain hands over the actions decided since the last call.
func (e *Engine) Drain() []Pending {
	out := e.outbox
	e.outbox = nil
	return out
}

// Resolve reports the results of sent actions, in order.
func (e *Engine) Resolve(sent []Pending, results []api.ActionResult) {
	for i, p := range sent {
		if i >= len(results) {
			return
		}
		res := results[i]
		if !res.OK {
			e.stats.actionFailed(p.Action.Type, res.Reason)
			e.logf("REFUSED %s: %s", p.Action.Type, res.Reason)
		}
		if p.done != nil {
			p.done(res)
		}
	}
}

// Handle answers the events of one step, oldest first, then runs the timed reviews.
func (e *Engine) Handle(events []api.Event) {
	for i := range events {
		e.handle(&events[i])
	}
	e.review()
}

func (e *Engine) handle(ev *api.Event) {
	switch ev.Kind {
	case api.KindPriceRequest:
		e.onRequest(ev.Request)
	case api.KindOrderWon:
		e.onWon(ev.Order, ev.At)
	case api.KindOrderLost:
		e.stats.Lost++
		e.onQuoteAnswer(ev.RequestID, false)
	case api.KindOrderChanged:
		e.onChanged(ev.Order, ev.Message)
	case api.KindOrderCancelled:
		e.onCancelled(ev)
	case api.KindGigViewed:
		e.onViewed(ev)
	case api.KindGigBooked:
		e.onBooked(ev)
	case api.KindGigDropped:
		e.onReopened(ev, false)
	case api.KindNoShow:
		e.onReopened(ev, true)
	case api.KindGigStarted:
		e.setGigStatus(ev.GigID, gigWorking)
	case api.KindDriverLate:
		e.onDriverLate(ev)
	case api.KindGigExpired:
		e.onExpired(ev)
	case api.KindPickedUp:
		e.onPickedUp(ev)
	case api.KindVisitFailed:
		e.onVisitFailed(ev)
	case api.KindDelivered:
		e.onDelivered(ev)
	case api.KindDeadlineClose:
		e.onDeadlineClose(ev)
	case api.KindLedger:
		e.onLedger(ev.Line)
	case api.KindOfferAnswered:
		e.onOfferAnswered(ev)
	case api.KindRelease, api.KindForecast:
		// Not acted on yet.
	}
}

func (e *Engine) setGigStatus(gigID, status string) {
	if g := e.gigs[gigID]; g != nil {
		g.status = status
	}
}

// NextWake is the next minute the engine wants to run even if no event comes: when an
// open gig stops being bookable, and at level 1 when a fee plan has its next step.
func (e *Engine) NextWake() int {
	wake := e.endsAt
	for _, g := range e.gigs {
		if g.status != gigOpen {
			continue
		}
		if g.bookableUntil >= e.now {
			wake = min(wake, g.bookableUntil+1)
		}
		if e.Level < levelSchedules {
			if next := nextStepAfter(g.plan, e.now); next > 0 {
				wake = min(wake, next)
			}
		}
	}
	for _, ob := range e.orders {
		if ob.cancelled {
			continue
		}
		for i := range ob.legs {
			if lb := &ob.legs[i]; !lb.delivered && lb.gig == nil && lb.retryAt > e.now {
				wake = min(wake, lb.retryAt)
			}
		}
	}
	return min(wake, e.nextOfferWake())
}

// review looks at every open gig: one that drivers can no longer book inside its windows
// is posted again, and at level 1 (one fee step per schedule) fees are raised to their plan.
func (e *Engine) review() {
	for _, g := range e.openGigs() {
		switch {
		case g.bookableUntil > 0 && e.now > g.bookableUntil:
			e.repost(g)
		case e.Level < levelSchedules && feeAt(g.plan, e.now) > g.shown:
			e.reprice(g)
		}
	}
	e.offerRound()
	e.adoptOrphans()
}

// adoptOrphans posts every car that has no gig although it still must move: one whose
// post was refused and held back, or lost in any other way. No car may be left behind:
// an undelivered car costs twice its price.
func (e *Engine) adoptOrphans() {
	if e.replay {
		return
	}
	for _, ob := range e.sortedOrders() {
		if ob.cancelled || ob.done() {
			continue
		}
		e.dispatch(ob)
	}
}

// openGigs lists the open gigs in posting order, so a run is the same every time.
func (e *Engine) openGigs() []*gigBook {
	var open []*gigBook
	for _, g := range e.gigs {
		if g.status == gigOpen {
			open = append(open, g)
		}
	}
	sort.Slice(open, func(i, j int) bool {
		if open[i].postedAt != open[j].postedAt {
			return open[i].postedAt < open[j].postedAt
		}
		return open[i].id < open[j].id
	})
	return open
}
