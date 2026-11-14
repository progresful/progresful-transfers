package engine

import (
	"progresful-transfers/internal/api"
)

// Gig statuses as the engine tracks them.
const (
	gigPending   = "pending" // post sent, id not known yet
	gigOpen      = "open"
	gigBooked    = "booked"
	gigWorking   = "working"
	gigDone      = "done"
	gigWithdrawn = "withdrawn"
)

const (
	stopPickup   = "pickup"
	stopDelivery = "delivery"
)

// legRef names one car of one order.
type legRef struct {
	orderID string
	leg     int
}

// legBook is what the engine knows about one car.
type legBook struct {
	gig        *gigBook // the live gig that carries the car, nil when it has none
	pickedUp   bool
	delivered  bool
	notBefore  int  // earliest minute for the next pickup try (after a failed visit)
	needManual bool // learned from a note or a failed visit
	needPlates bool
	fails      int
	// retryAt holds back a new post after the world refused one; plain posts the car with
	// no window of its own, which the world always takes.
	retryAt int
	plain   bool
}

// orderBook is one won order with the request it came from.
type orderBook struct {
	order     api.Order
	req       *api.PriceRequest
	legs      []legBook
	wonAt     int
	urgent    bool // its deadline warning has come
	cancelled bool
	refunded  bool // a late refund or penalty hit the account
	paid      bool // revenue came in
	// noteDeadline is a deadline from the dealer's note (0: none).
	noteDeadline int
	// narrowed: the note gives a site shorter hours than stated; the car is posted day by day.
	narrowed bool
	// chainTries counts the chain attempts the order has been in; after ChainTries it goes alone.
	chainTries int
	// chainAt is when the order's latest chain attempt was posted (0: never in one).
	chainAt int
	// rushed: the car was posted once to arrive on the day it was ready (rush.go).
	rushed bool
	// trustCounted / trustLate: how the order entered its dealer's trust count.
	trustCounted bool
	trustLate    bool
	// For the per-order report.
	firstBookedAt int
	feesPaid      int
	bookings      int
	drops         int
}

func (o *orderBook) undelivered() []int {
	var legs []int
	for i := range o.legs {
		if !o.legs[i].delivered {
			legs = append(legs, i)
		}
	}
	return legs
}

func (o *orderBook) done() bool { return len(o.undelivered()) == 0 }

// gigBook is one gig the engine posted.
type gigBook struct {
	id       string
	legs     []legRef
	post     api.Action    // the post that made it (stops, audience)
	plan     []api.FeeStep // the full fee plan; at level 1 only its current step is in the app
	shown    int           // the fee the app shows now, as far as the engine knows
	base     int           // base fee: the typical fee of its cars
	feeMult  float64       // expected booking fee as a multiple of base (fee model); 0: unknown
	status   string
	driver   string
	fee      int // booked fee
	postedAt int
	mode     int // posting mode: buffered, tight or late
	buffer   int // minutes the delivery window ends before the deadline (buffered mode)
	// chainUntil is when a chain attempt (modeChain) ends and its cars go alone; chainLast
	// is the last minute its first car can be collected with both cars still on time.
	chainUntil int
	chainLast  int
	// clockFrom is the minute the gig's fee clock counts from when that is before the
	// post: a car that already waited in a chain attempt does not open at the lowest fee.
	clockFrom int
	// dayLate marks a day-window gig (modeDay) whose day is after the deadline.
	dayLate bool
	// bookableUntil is the last minute a driver can still book the gig inside its windows.
	// After it the gig is dead weight in the app and must be posted again. 0 for a late gig.
	bookableUntil int
	views         int
	pickedAt      int            // minute the gig's latest car was collected (0: none yet)
	viewers       map[string]int // driver id -> minute the driver last opened the gig
	offers        map[string]int // driver id -> minute an open direct offer was sent
	offerFee      int            // fee of the latest offers
}

func (g *gigBook) live() bool {
	return g.status == gigPending || g.status == gigOpen || g.status == gigBooked || g.status == gigWorking
}

func (g *gigBook) carries(ref legRef) bool {
	for _, l := range g.legs {
		if l == ref {
			return true
		}
	}
	return false
}

// Pending is an action waiting to be sent, with what to do with its result.
type Pending struct {
	Action api.Action
	done   func(api.ActionResult)
}
