package engine

import (
	"sort"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

const (
	offerLifeMin          = 10
	maxOpenOffers         = 3
	maxOffersPerDriverDay = 3
	quietFromMin          = 23 * cal.Hour // no driver reads an offer from 23:00 to 07:00
	quietToMin            = 7 * cal.Hour
	notifyRadiusKm        = 30.0
	experiencedRating     = 4.8
	experiencedGigs       = 50
	licenceManual         = "manual"
	answerAccepted        = "accepted"
	answerDeclined        = "declined"
	// positionFreshMin is how long a driver is assumed to stay where a gig left them.
	positionFreshMin = 180
	// Penalties (minutes of travel) for a driver the engine has not seen in the app lately.
	notSeenTodayPenalty = 90
	neverSeenPenalty    = 240
)

// driverBook is what the engine knows about one driver beyond the public profile.
type driverBook struct {
	profile    api.DriverProfile
	lastSeen   int // last minute the driver viewed or booked one of our gigs (-1: never)
	offerDay   int // day index the offer count belongs to
	offersSent int // direct offers sent on offerDay
	place      string
	placeAt    int             // when the driver was last known to be at place
	declined   map[string]int  // order id -> highest fee the driver turned down
	offered    map[string]bool // gig id -> an offer of that gig was already sent
}

func (e *Engine) driver(id string) *driverBook {
	d := e.drivers[id]
	if d == nil {
		d = &driverBook{profile: api.DriverProfile{ID: id}, lastSeen: -1, declined: map[string]int{}, offered: map[string]bool{}}
		e.drivers[id] = d
	}
	return d
}

func (e *Engine) sawDriver(id string, at int) {
	if id == "" {
		return
	}
	d := e.driver(id)
	d.lastSeen = max(d.lastSeen, at)
}

func (e *Engine) driverAt(id, place string, at int) {
	if id == "" || place == "" {
		return
	}
	d := e.driver(id)
	d.place, d.placeAt = place, at
}

// whereabouts is where a driver most likely is now: where a gig of ours left them, if
// that is recent, else at home.
func (e *Engine) whereabouts(d *driverBook) string {
	if d.place != "" && e.now-d.placeAt <= positionFreshMin {
		return d.place
	}
	return d.profile.HomePlaceID
}

func (e *Engine) offersLeft(d *driverBook) int {
	if d.offerDay != cal.DayOf(e.now) {
		return maxOffersPerDriverDay
	}
	return maxOffersPerDriverDay - d.offersSent
}

func quietHours(now int) bool {
	m := cal.MinuteOfDay(now)
	return m >= quietFromMin || m < quietToMin
}

// notifyReach counts the drivers the app notifies of a gig with this first pickup: those
// who live within 30 km of it, in a straight line.
func (e *Engine) notifyReach(place string) int { return e.driversNear(place, false) }

// wantsOffers decides whether an open gig should get direct offers now: when few
// drivers hear of it by themselves, or it has waited long enough in the app.
func (e *Engine) wantsOffers(g *gigBook) bool {
	if e.P.Offers != 1 || e.Level < levelNetwork || g.status != gigOpen || g.id == "" || len(g.legs) == 0 || quietHours(e.now) {
		return false
	}
	ob := e.orders[g.legs[0].orderID]
	if ob == nil {
		return false
	}
	if e.now-g.postedAt >= e.P.OfferAfterMin {
		return true
	}
	return e.notifyReach(ob.order.Legs[g.legs[0].leg].Pickup.PlaceID) < e.P.OfferReach
}

func (g *gigBook) openOffers(now int) int {
	n := 0
	for _, at := range g.offers {
		if now-at < offerLifeMin {
			n++
		}
	}
	return n
}

// offerRound sends direct offers for the gigs that want them, up to three open per gig.
func (e *Engine) offerRound() {
	if e.P.Offers != 1 || e.Level < levelNetwork || quietHours(e.now) {
		return
	}
	for _, g := range e.openGigs() {
		if !e.wantsOffers(g) {
			continue
		}
		room := maxOpenOffers - g.openOffers(e.now)
		if room <= 0 {
			continue
		}
		fee := max(feeAt(g.plan, e.now), feeOf(g.base, e.P.OfferMult))
		for _, d := range e.candidates(g, fee, room) {
			e.offer(g, d, fee)
		}
	}
}

type candidate struct {
	d     *driverBook
	score int
}

// candidates ranks the drivers who could take the gig: by the travel it costs them (to
// the pickup from where they are, and home from the last delivery), with a penalty for
// drivers not seen in the app lately.
func (e *Engine) candidates(g *gigBook, fee, n int) []*driverBook {
	first, last := g.post.Stops[0], g.post.Stops[len(g.post.Stops)-1]
	pickupOrder, deliveryOrder := e.orders[first.OrderID], e.orders[last.OrderID]
	if pickupOrder == nil || deliveryOrder == nil {
		return nil
	}
	pickup := pickupOrder.order.Legs[first.Leg].Pickup.PlaceID
	delivery := deliveryOrder.order.Legs[last.Leg].Delivery.PlaceID
	today := cal.DayOf(e.now)
	var found []candidate
	for _, d := range e.drivers {
		if e.offersLeft(d) <= 0 || d.offered[g.id] || d.declined[first.OrderID] >= fee {
			continue
		}
		if g.post.Audience.ManualLicence && d.profile.Licence != licenceManual {
			continue
		}
		if g.post.Audience.Experienced && (d.profile.Rating < experiencedRating || d.profile.CompletedGigs < experiencedGigs) {
			continue
		}
		approach := e.Map.Minutes(e.whereabouts(d), pickup)
		if g.bookableUntil > 0 && e.now+approach > g.bookableUntil {
			continue // cannot be at the pickup in time
		}
		score := approach + e.Map.Minutes(delivery, d.profile.HomePlaceID)
		switch {
		case d.lastSeen < 0:
			score += neverSeenPenalty
		case cal.DayOf(d.lastSeen) != today:
			score += notSeenTodayPenalty
		}
		if score <= e.P.OfferMaxScore {
			found = append(found, candidate{d: d, score: score})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score < found[j].score
		}
		return found[i].d.profile.ID < found[j].d.profile.ID
	})
	out := make([]*driverBook, 0, n)
	for i := 0; i < len(found) && i < n; i++ {
		out = append(out, found[i].d)
	}
	return out
}

func (e *Engine) offer(g *gigBook, d *driverBook, fee int) {
	if day := cal.DayOf(e.now); d.offerDay != day {
		d.offerDay, d.offersSent = day, 0
	}
	d.offersSent++
	d.offered[g.id] = true
	if g.offers == nil {
		g.offers = map[string]int{}
	}
	g.offers[d.profile.ID] = e.now
	g.offerFee = fee
	e.stats.Offers++
	id := d.profile.ID
	e.send(api.Action{Type: api.ActOffer, GigID: g.id, DriverID: id, FeeCents: fee}, func(res api.ActionResult) {
		if !res.OK {
			delete(g.offers, id) // refused: it holds no offer slot of the gig
		}
	})
}

// onOfferAnswered frees the gig's offer slot and remembers a refusal.
func (e *Engine) onOfferAnswered(ev *api.Event) {
	e.stats.OfferAnswers[ev.Answer]++
	d := e.driver(ev.DriverID)
	g := e.gigs[ev.GigID]
	if g == nil {
		return
	}
	delete(g.offers, ev.DriverID)
	switch ev.Answer {
	case answerAccepted:
		e.sawDriver(ev.DriverID, ev.At)
	case answerDeclined:
		// The driver read it and said no: active today, but this fee does not pay them.
		e.sawDriver(ev.DriverID, ev.At)
		if len(g.legs) > 0 {
			d.declined[g.legs[0].orderID] = max(d.declined[g.legs[0].orderID], g.offerFee)
		}
	}
}

// nextOfferWake is the next minute at which a gig may start to want offers.
func (e *Engine) nextOfferWake() int {
	wake := e.endsAt
	if e.P.Offers != 1 || e.Level < levelNetwork {
		return wake
	}
	for _, g := range e.gigs {
		if g.status != gigOpen || len(g.offers) > 0 {
			continue
		}
		if at := g.postedAt + e.P.OfferAfterMin; at > e.now {
			wake = min(wake, at)
		}
	}
	if wake < e.endsAt && quietHours(wake) {
		// Wait for the morning: nobody reads an offer at night.
		day := cal.DayOf(wake)
		if cal.MinuteOfDay(wake) >= quietFromMin {
			day++
		}
		wake = cal.At(day, quietToMin)
	}
	return wake
}
