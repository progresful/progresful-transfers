package notes

// saturday is the first weekday index of the weekend (Monday is 0).
const saturday = 5

type factKind uint8

const (
	kNone factKind = iota
	kNotBefore
	kDeadline
	kPickupHours
	kPickupOpens
	kPickupCloses
	kDeliveryHours
	kDeliveryOpens
	kDeliveryCloses
)

// assertion is one thing the note states, before days are made absolute.
type assertion struct {
	kind factKind
	expr timeExpr
}

// extract reads the clauses in order. A clause with a time becomes an
// assertion; a clause that starts with a correction marker changes the
// assertion before it.
func extract(segs []segment) []assertion {
	var out []assertion
	var carry feat
	carrySentence := None
	for i := range segs {
		seg := &segs[i]
		exprs, negBefore, negFree := scanTimes(seg)
		f := features(seg)
		if negBefore {
			f |= fEarliest
		}
		if negFree && f&fAfter != 0 {
			f |= fNobody // "ei noutoja klo 15 jalkeen", "no pickups after 3pm"
		}
		live := undenied(exprs)
		if seg.corr && len(out) > 0 {
			out = correct(out, f, live)
			continue
		}
		timed := withTime(live)
		if f&fCall != 0 { // "call 15 min before you arrive"
			carry = 0
			continue
		}
		if len(timed) == 0 {
			if carrySentence != seg.sentence {
				carry, carrySentence = 0, seg.sentence
			}
			carry |= f
			continue
		}
		if f&fTopic == 0 && carrySentence == seg.sentence {
			f |= carry // "luovutus vasta kun maksettu, aikaisintaan klo 12"
		}
		for _, e := range timed {
			if k := classify(f, e); k != kNone {
				out = append(out, assertion{kind: k, expr: e})
			}
		}
		carry, carrySentence = 0, None
	}
	return out
}

func features(seg *segment) feat {
	var f feat
	for i := range seg.toks {
		if seg.toks[i].kind == kWord {
			f |= seg.toks[i].f
		}
	}
	return f
}

func undenied(exprs []timeExpr) []timeExpr {
	var out []timeExpr
	for _, e := range exprs {
		if !e.denied {
			out = append(out, e)
		}
	}
	return out
}

func withTime(exprs []timeExpr) []timeExpr {
	var out []timeExpr
	for _, e := range exprs {
		if e.start != None {
			out = append(out, e)
		}
	}
	return out
}

// correct applies a correcting clause to the assertions so far and returns
// the new list. A bare day or time ("eikun klo 11.30", "Korjaus: maanantaina")
// replaces that part of the last assertion. A correction that names its own
// subject replaces the last assertion of that kind.
func correct(out []assertion, f feat, live []timeExpr) []assertion {
	if len(live) == 0 {
		return out
	}
	c := live[0]
	next := append([]assertion(nil), out...)
	if f&fTopic != 0 && c.start != None {
		if k := classify(f, c); k != kNone {
			for j := len(next) - 1; j >= 0; j-- {
				if next[j].kind == k {
					next[j] = assertion{kind: k, expr: c}
					return next
				}
			}
			return append(next, assertion{kind: k, expr: c})
		}
	}
	last := len(next) - 1
	next[last] = assertion{kind: next[last].kind, expr: merged(next[last].expr, c)}
	return next
}

// merged returns e with the parts that the correction c states replaced.
func merged(e, c timeExpr) timeExpr {
	if c.hasDay() {
		e.day, e.wd = c.day, c.wd
	}
	switch {
	case c.isRange():
		if e.isRange() {
			e.start, e.end = c.start, c.end
		}
	case c.start != None && e.isRange():
		if c.start > e.start {
			e.end = c.start
		} else {
			e.start = c.start
		}
	case c.start != None:
		e.start = c.start
	}
	return e
}

// classify decides what a time in a clause with features f means.
func classify(f feat, e timeExpr) factKind {
	if f&fSeller != 0 {
		return kNone // "Myyja kotona 9-12": the booked slot of a private address
	}
	dest := f&fDest != 0
	if e.isRange() {
		switch {
		case e.end <= e.start, e.wd >= saturday:
			return kNone // not a span of hours, or weekend hours: Facts has no place for those
		case dest, f&fArrive != 0:
			return kDeliveryHours
		case f&(fOpen|fOpens|fClose|fPickup|fWeekdays) != 0:
			return kPickupHours
		}
		return kNone
	}
	limit := f&(fUntil|fBefore|fLatest) != 0
	if f&(fNobody|fOpens|fClose|fOpen) != 0 {
		// Hours of a site: the delivery site when the clause speaks of the
		// destination or the delivery, else the pickup site.
		closes, opens := kPickupCloses, kPickupOpens
		if f&(fDest|fArrive) != 0 {
			closes, opens = kDeliveryCloses, kDeliveryOpens
		}
		switch {
		case f&fNobody != 0: // "Taalla ei ketaan neljan jalkeen"
			return closes
		case f&fOpens != 0, f&fClose != 0 && f&fUntil != 0: // "opens at 9", "closed until 10"
			return opens
		case f&fClose != 0, limit:
			return closes
		case f&fEarliest != 0, e.start < noonHour*minutesPerHr: // "auki klo 9 alkaen", "open at 10"
			return opens
		}
		return closes
	}
	closes, opens := kPickupCloses, kPickupOpens
	if dest {
		closes, opens = kDeliveryCloses, kDeliveryOpens
	}
	switch {
	case f&fPay != 0:
		return kNotBefore
	case f&fArrive != 0 && f&fUntil != 0:
		return kDeliveryCloses
	case f&fArrive != 0 && f&fEarliest != 0 && !limit: // "delivery from 10:00"
		return kDeliveryOpens
	case f&fArrive != 0:
		return kDeadline
	case f&fCustomer != 0 && f&fPickup != 0: // "Asiakas noutaa huomenna 20.00"
		return kDeadline
	case dest:
		if f&fEarliest != 0 && !limit {
			return opens
		}
		return closes
	case f&(fEarliest|fReady|fAfter) != 0:
		return kNotBefore
	case f&(fBefore|fUntil) != 0:
		return kPickupCloses
	case f&fLatest != 0:
		if f&fPickup != 0 {
			return kPickupCloses
		}
		return kDeadline
	case f&fPickup != 0 && e.hasDay():
		return kNotBefore
	}
	return kNone
}

// assemble turns the assertions into Facts. When a note states the same kind
// of fact twice with different values, the stricter one is kept.
func assemble(as []assertion, arrivedAt int) Facts {
	f := empty()
	for _, a := range as {
		e := a.expr
		switch a.kind {
		case kNotBefore:
			f.NotBefore = later(f.NotBefore, absolute(e, arrivedAt))
		case kDeadline:
			f.Deadline = earlier(f.Deadline, absolute(e, arrivedAt))
		case kPickupHours:
			f.PickupOpens = later(f.PickupOpens, e.start)
			f.PickupCloses = earlier(f.PickupCloses, e.end)
		case kPickupOpens:
			f.PickupOpens = later(f.PickupOpens, e.start)
		case kPickupCloses:
			f.PickupCloses = earlier(f.PickupCloses, e.start)
		case kDeliveryHours:
			f.DeliveryOpens = later(f.DeliveryOpens, e.start)
			f.DeliveryCloses = earlier(f.DeliveryCloses, e.end)
		case kDeliveryOpens:
			f.DeliveryOpens = later(f.DeliveryOpens, e.start)
		case kDeliveryCloses:
			f.DeliveryCloses = earlier(f.DeliveryCloses, e.start)
		}
	}
	return f
}

func later(a, b int) int {
	if a == None || b > a {
		return b
	}
	return a
}

func earlier(a, b int) int {
	if a == None || b < a {
		return b
	}
	return a
}

// absolute places a time of day on its day, counted from the arrival day. A
// weekday name is that weekday after the arrival day, never the arrival day.
func absolute(e timeExpr, arrivedAt int) int {
	day := arrivedAt / minutesPerDay
	if arrivedAt < 0 && arrivedAt%minutesPerDay != 0 {
		day--
	}
	switch {
	case e.wd != None:
		ahead := ((e.wd-day)%daysPerWeek + daysPerWeek) % daysPerWeek
		if ahead == 0 {
			ahead = daysPerWeek
		}
		day += ahead
	case e.day != None:
		day += e.day
	}
	return day*minutesPerDay + e.start
}
