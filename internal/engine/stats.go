package engine

import (
	"encoding/json"
	"math"
	"strings"
)

// reasonKeyLen cuts refusal reasons to a short key for counting.
const reasonKeyLen = 48

// Stats counts what happened in a run, for the run index.
type Stats struct {
	Requests         int            `json:"requests"`
	Quoted           int            `json:"quoted"`
	Declined         int            `json:"declined"`
	DeclinedSupply   int            `json:"declinedSupply"`
	RiskyOptions     int            `json:"riskyOptions"`
	Requotes         int            `json:"requotes"` // quotes sent again before their deadline
	DeclinedNote     map[string]int `json:"declinedNote"`
	Won              int            `json:"won"`
	Lost             int            `json:"lost"`
	Cancelled        int            `json:"cancelled"`
	Changed          int            `json:"changed"`
	Delivered        int            `json:"delivered"`
	LateLegs         int            `json:"lateLegs"`
	Posts            int            `json:"posts"`
	PostFailures     int            `json:"postFailures"`
	Reprices         int            `json:"reprices"`
	Reposts          int            `json:"reposts"`
	Chains           int            `json:"chains"`
	ChainBookings    int            `json:"chainBookings"`
	LongChains       int            `json:"longChains"`
	ChainsSkipped    int            `json:"chainsSkipped"`
	ChainProspects   int            `json:"chainProspects"`
	Duplicates       int            `json:"duplicates"`
	Offers           int            `json:"offers"`
	ChainOffers      int            `json:"chainOffers"`
	OfferAnswers     map[string]int `json:"offerAnswers"`
	Views            int            `json:"views"`
	Bookings         int            `json:"bookings"`
	Drops            int            `json:"drops"`
	NoShows          int            `json:"noShows"`
	DriverLate       int            `json:"driverLate"`
	Expired          int            `json:"expired"`
	DeadlineWarnings int            `json:"deadlineWarnings"`
	BookedFeeCents   int            `json:"bookedFeeCents"`
	BookedBaseCents  int            `json:"bookedBaseCents"`
	BookWaitMin      int            `json:"bookWaitMin"`
	FailedVisits     map[string]int `json:"failedVisits"`
	Ledger           map[string]int `json:"ledger"`
	ActionFailures   map[string]int `json:"actionFailures"`
}

func newStats() Stats {
	return Stats{FailedVisits: map[string]int{}, Ledger: map[string]int{}, ActionFailures: map[string]int{}, OfferAnswers: map[string]int{}, DeclinedNote: map[string]int{}}
}

func (s *Stats) booked(g *gigBook, at int) {
	s.Bookings++
	if g.mode == modeChain {
		s.ChainBookings++
	}
	s.BookedFeeCents += g.fee
	s.BookedBaseCents += g.base
	s.BookWaitMin += at - g.postedAt
}

func (s *Stats) actionFailed(kind, reason string) {
	key := reason
	// Reasons name ids and times; keep the stable tail for counting.
	if i := strings.Index(key, ": "); i >= 0 && i+2 < len(key) {
		key = key[i+2:]
	}
	if len(key) > reasonKeyLen {
		key = key[:reasonKeyLen]
	}
	s.ActionFailures[kind+": "+key]++
}

// Stats returns a copy of the run's counters as a plain map for the run log.
func (e *Engine) Stats() map[string]any {
	out := map[string]any{}
	raw, err := json.Marshal(e.stats)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	out["marketRatio"] = math.Round(1000*e.market.worldRatio()) / 1000
	trusted := 0
	for _, d := range e.market.dealers {
		trusted += d.trust
	}
	out["trustPoints"] = trusted
	out["costMultNow"] = math.Round(100*e.costMult()) / 100
	out["onTimeOutlook"] = math.Round(1000*e.onTimeOutlook()) / 1000
	return out
}

// OrderLine is one won order in the per-order report of a run.
type OrderLine struct {
	ID            string  `json:"id"`
	Dealer        string  `json:"dealer"`
	Option        string  `json:"option"`
	Legs          int     `json:"legs"`
	Pickup        string  `json:"pickup"`
	Delivery      string  `json:"delivery"`
	Km            float64 `json:"km"`
	DriveMin      int     `json:"driveMin"`
	PriceCents    int     `json:"priceCents"`
	RefCents      int     `json:"refCents"`
	WonAt         int     `json:"wonAt"`
	Deadline      int     `json:"deadline"`
	LastPickupAt  int     `json:"lastPickupAt"`
	Bookings      int     `json:"bookings"`
	Drops         int     `json:"drops"`
	FirstBookedAt int     `json:"firstBookedAt"`
	CostCents     int     `json:"costCents"`
	Delivered     bool    `json:"delivered"`
	Refunded      bool    `json:"refunded"`
	Cancelled     bool    `json:"cancelled"`
}

// OrderReport lists every won order with its money and timing.
func (e *Engine) OrderReport() []OrderLine {
	lines := make([]OrderLine, 0, len(e.orders))
	for _, ob := range e.orders {
		leg := &ob.order.Legs[0]
		lines = append(lines, OrderLine{
			ID: ob.order.ID, Dealer: ob.order.DealerID, Option: ob.order.Option, Legs: len(ob.order.Legs),
			Pickup: leg.Pickup.PlaceID, Delivery: leg.Delivery.PlaceID,
			Km: e.Map.Km(leg.Pickup.PlaceID, leg.Delivery.PlaceID), DriveMin: e.driveMin(leg),
			PriceCents: ob.order.PriceCents, RefCents: ob.req.ReferenceFeeCents,
			WonAt: ob.wonAt, Deadline: ob.order.Deadline, LastPickupAt: ob.order.LastPickupAt,
			Bookings: ob.bookings, Drops: ob.drops, FirstBookedAt: ob.firstBookedAt, CostCents: ob.feesPaid,
			Delivered: ob.done(), Refunded: ob.refunded, Cancelled: ob.cancelled,
		})
	}
	return lines
}

// declineCounts are the counters that pricing a request moves; a request that is priced
// a second time (requote.go) must not count twice.
type declineCounts struct {
	supply, risky, prospects int
	notes                    map[string]int
}

func (s *Stats) snapshotDeclines() declineCounts {
	c := declineCounts{supply: s.DeclinedSupply, risky: s.RiskyOptions, prospects: s.ChainProspects, notes: make(map[string]int, len(s.DeclinedNote))}
	for k, v := range s.DeclinedNote {
		c.notes[k] = v
	}
	return c
}

func (s *Stats) restoreDeclines(c declineCounts) {
	s.DeclinedSupply, s.RiskyOptions, s.ChainProspects = c.supply, c.risky, c.prospects
	s.DeclinedNote = c.notes
}
