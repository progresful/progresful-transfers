package notes

const (
	noonHour     = 12
	maxClockHour = 24
	minutesPerHr = 60
	halfHour     = 30
	negReach     = 3 // "not ready before", "ei saa hakea ennen": words between a negation and "before"
)

// timeExpr is one day and/or time of day read from a clause.
type timeExpr struct {
	day    int // days after the arrival day, or None
	wd     int // weekday 0..6 when the day is named, or None
	start  int // minute of day, or None for a day without a time
	end    int // minute of day closing a range, or None
	denied bool
}

func (e timeExpr) hasDay() bool  { return e.day != None || e.wd != None }
func (e timeExpr) isRange() bool { return e.start != None && e.end != None }

// scanner walks one clause and collects its time expressions.
type scanner struct {
	toks      []token
	exprs     []timeExpr
	day, wd   int  // a day waiting for its time
	dayDenied bool // that day was denied
	denyAt    int  // index of a negation that denies the expression right after it ("ei klo 14", "not 3pm"), or None
	negBefore bool // "not before", "ei ennen": an earliest time
	negFree   bool // a negation that neither denies a time nor makes "not before"
	corr      bool
}

// scanTimes reads every day and time in the clause. It also clears features
// of words it used up as part of a time ("to" in "9 to 5", "after" in "the day
// after tomorrow"), so they do not colour the meaning of the clause.
func scanTimes(seg *segment) (exprs []timeExpr, negBefore, negFree bool) {
	s := &scanner{toks: seg.toks, day: None, wd: None, denyAt: None, corr: seg.corr}
	for i := 0; i < len(s.toks); {
		i = s.step(i)
	}
	if s.day != None || s.wd != None {
		s.exprs = append(s.exprs, timeExpr{day: s.day, wd: s.wd, start: None, end: None, denied: s.dayDenied})
	}
	return s.exprs, s.negBefore, s.negFree
}

// denied reports whether the expression starting at token i follows a
// negation directly, with only day and clock words in between.
func (s *scanner) denied(i int) bool {
	if s.denyAt == None {
		return false
	}
	for j := s.denyAt + 1; j < i; j++ {
		if !s.has(j, fClock|fDay|fWeekday) {
			return false
		}
	}
	return true
}

func (s *scanner) at(i int) *token {
	if i < 0 || i >= len(s.toks) {
		return nil
	}
	return &s.toks[i]
}

func (s *scanner) has(i int, f feat) bool {
	t := s.at(i)
	return t != nil && t.kind == kWord && t.f&f != 0
}

// startsTime reports whether a clock time can begin at token i.
func (s *scanner) startsTime(i int) bool {
	t := s.at(i)
	if t == nil {
		return false
	}
	if t.kind == kNum {
		return t.digits <= maxRefDigits
	}
	return t.kind == kWord && t.f&(fClock|fHalf|fHour) != 0
}

func (s *scanner) step(i int) int {
	t := &s.toks[i]
	if t.kind == kNum {
		if m, next, ok := s.clock(i); ok {
			return s.finish(i, m, next)
		}
		return i + 1
	}
	if t.kind != kWord {
		return i + 1
	}
	switch {
	case t.f&fNeg != 0:
		s.negation(i)
	case t.f&fDay != 0:
		s.relativeDay(i)
	case t.f&fWeekday != 0:
		return s.weekday(i)
	case t.f&fHalf != 0:
		if m, next, ok := s.half(i); ok {
			return s.finish(i, m, next)
		}
	case t.f&fHour != 0:
		if s.hourWordCounts(i) {
			return s.finish(i, workHour(t.val)*minutesPerHr, i+1)
		}
	}
	return i + 1
}

// negation decides what "ei", "not", "inte" before token i+1 means.
func (s *scanner) negation(i int) {
	if s.has(i+1, fDay|fWeekday) || s.startsTime(i+1) {
		s.denyAt = i
		return
	}
	for j := i + 1; j <= i+negReach; j++ {
		t := s.at(j)
		if t == nil || t.kind != kWord {
			break
		}
		if t.f&(fBefore|fUntil) != 0 { // "not before", "ei ennen", "not ready before"
			s.negBefore = true
			t.f &^= fBefore | fUntil
			return
		}
	}
	s.negFree = true
}

func (s *scanner) relativeDay(i int) {
	day := s.toks[i].val
	if a, d := s.at(i-1), s.at(i-2); a != nil && d != nil && a.text == "after" && d.text == "day" && day == 1 {
		day = 2 // "the day after tomorrow"
		a.f = 0
	}
	s.setDay(i, day, None)
}

// weekday handles "torstaina", "to.", "Thurs", and skips "ma-pe" style spans.
func (s *scanner) weekday(i int) int {
	t := &s.toks[i]
	if d, w := s.at(i+1), s.at(i+2); d != nil && w != nil && d.kind == kDash && w.kind == kWord && w.f&fWeekday != 0 {
		t.f = fWeekdays
		return i + 3 // "mon-fri": every weekday, not a day
	}
	if t.f&fShort != 0 && !s.startsTime(i+1) && !s.corr && !s.denied(i) {
		return i + 1 // "to", "ma", "sun" as ordinary words
	}
	s.setDay(i, None, t.val)
	return i + 1
}

// setDay attaches a day to the time just read ("klo 14 huomenna") or keeps it
// for the time that follows.
func (s *scanner) setDay(i, day, wd int) {
	if n := len(s.exprs); n > 0 && s.day == None && s.wd == None && !s.exprs[n-1].hasDay() && s.exprs[n-1].start != None && !s.startsTime(i+1) {
		s.exprs[n-1].day, s.exprs[n-1].wd = day, wd
		return
	}
	s.day, s.wd, s.dayDenied = day, wd, s.denied(i)
}

// finish stores a time of day that starts at token at and ends before token
// next, with an optional second time that makes it a range.
func (s *scanner) finish(at, start, next int) int {
	e := timeExpr{day: s.day, wd: s.wd, start: start, end: None, denied: s.dayDenied || s.denied(at)}
	s.day, s.wd, s.dayDenied, s.denyAt = None, None, false, None
	if end, after, ok := s.rangeEnd(next); ok {
		e.end = end
		next = after
		if e.end <= e.start && e.end+noonHour*minutesPerHr > e.start && e.end < noonHour*minutesPerHr {
			e.end += noonHour * minutesPerHr // "9-5"
		}
	}
	s.exprs = append(s.exprs, e)
	return next
}

// rangeEnd reads the closing time of "9-17", "9 to 17", "9:sta 20:een".
func (s *scanner) rangeEnd(i int) (int, int, bool) {
	j := i
	sep := s.at(j)
	if sep == nil {
		return 0, 0, false
	}
	prevSuffix := i > 0 && s.toks[i-1].kind == kNum && s.toks[i-1].suffix
	switch {
	case sep.kind == kDash, sep.kind == kWord && sep.f&fRangeSep != 0:
		j++
	case sep.kind == kNum && sep.suffix && prevSuffix:
	case sep.kind == kWord && sep.f&fHour != 0 && s.has(i-1, fHour):
	default:
		return 0, 0, false
	}
	if s.has(j, fClock) {
		j++
	}
	t := s.at(j)
	if t == nil {
		return 0, 0, false
	}
	if t.kind == kWord && t.f&fHour != 0 {
		sep.f &^= fRangeSep | fUntil | fWeekday
		return workHour(t.val) * minutesPerHr, j + 1, true
	}
	if t.kind != kNum {
		return 0, 0, false
	}
	m, next, ok := s.clockAt(j, true)
	if !ok {
		return 0, 0, false
	}
	if sep.kind == kWord {
		sep.f &^= fRangeSep | fUntil | fWeekday
	}
	return m, next, true
}

// half reads "puoli kymmenelta" (9:30), "halv tre", "half three" (14:30) and
// "half past two" (14:30).
func (s *scanner) half(i int) (int, int, bool) {
	j := i + 1
	past := false
	if t := s.at(j); t != nil && t.kind == kWord && t.text == "past" {
		past = true
		j++
	}
	t := s.at(j)
	if t == nil {
		return 0, 0, false
	}
	hour := 0
	switch {
	case t.kind == kWord && t.f&fHour != 0:
		hour = workHour(t.val)
	case t.kind == kNum && t.min == None && t.num >= 1 && t.num <= noonHour:
		hour = workHour(t.num)
	default:
		return 0, 0, false
	}
	if past {
		return hour*minutesPerHr + halfHour, j + 1, true
	}
	return hour*minutesPerHr - halfHour, j + 1, true
}

// hourWordCounts reports whether the hour word at i is a clock time and not a
// count ("yksi siirto", "two sets of tyres").
func (s *scanner) hourWordCounts(i int) bool {
	if s.has(i+1, fNoTimeNext) {
		return false
	}
	if s.toks[i].f&fStrong != 0 {
		return true
	}
	const around = fClock | fBefore | fUntil | fAfter | fLatest | fEarliest | fDay | fWeekday
	if s.has(i-1, around) || s.has(i+1, fUntil|fAfter|fEarliest|fLatest) {
		return true
	}
	next := s.at(i + 1)
	return next != nil && next.kind == kWord && (next.text == "oclock" || next.text == "o")
}

// clock reads the number at i as a time of day when its surroundings say it
// is one. Bay numbers, key-box codes, "15 min" and mileages are not.
func (s *scanner) clock(i int) (int, int, bool) {
	return s.clockAt(i, false)
}

func (s *scanner) clockAt(i int, inRange bool) (int, int, bool) {
	t := &s.toks[i]
	if t.digits > maxRefDigits || s.has(i-1, fNoTimePrev) {
		return 0, 0, false
	}
	next := i + 1
	if s.has(next, fNoTimeNext) {
		return 0, 0, false
	}
	hour, minute := t.num, 0
	sure := inRange || t.suffix || s.has(i-1, fClock)
	switch {
	case t.min != None:
		minute = t.min
		sure = true
	case t.digits > 2: // "kello 1400"
		if !s.has(i-1, fClock) && !inRange {
			return 0, 0, false
		}
		hour, minute = t.num/100, t.num%100
	}
	if hour > maxClockHour || minute >= minutesPerHr || (hour == maxClockHour && minute > 0) {
		return 0, 0, false
	}
	if s.has(next, fMeridiem) {
		hour = meridiem(hour, s.toks[next].val == 1)
		next++
		sure = true
	} else if pm, ok := s.rangeMeridiem(next); ok && pm && hour < noonHour {
		// "1-3pm": the first hour is an afternoon hour too when that keeps the range in order.
		if end := s.toks[next+1].num; end < noonHour && hour < end {
			hour += noonHour
		}
	}
	if !sure && !s.bareNumberIsTime(i, next) {
		return 0, 0, false
	}
	return hour*minutesPerHr + minute, next, true
}

// rangeMeridiem looks past a dash for "<number> pm".
func (s *scanner) rangeMeridiem(i int) (bool, bool) {
	d, n := s.at(i), s.at(i+1)
	if d == nil || n == nil || d.kind != kDash || n.kind != kNum || !s.has(i+2, fMeridiem) {
		return false, false
	}
	return s.toks[i+2].val == 1, true
}

// bareNumberIsTime: a plain "15" is a time only next to a time word, a day,
// or as the start of a range.
func (s *scanner) bareNumberIsTime(i, next int) bool {
	const before = fClock | fDay | fWeekday | fBefore | fUntil | fAfter | fLatest | fEarliest | fHalf
	if s.has(i-1, before) || s.has(next, fUntil|fAfter|fEarliest|fLatest) {
		return true
	}
	sep, n := s.at(next), s.at(next+1)
	if sep == nil || n == nil || n.kind != kNum || n.digits > 2 {
		return false
	}
	if sep.kind == kDash {
		return true
	}
	return sep.kind == kWord && sep.f&fRangeSep != 0 && !s.has(next+2, fNoTimeNext)
}

func meridiem(hour int, pm bool) int {
	switch {
	case pm && hour < noonHour:
		return hour + noonHour
	case !pm && hour == noonHour:
		return 0
	}
	return hour
}

// workHour maps an hour number written in words to the working hour it means:
// "two" is 14:00, "kymmenelta" is 10:00.
func workHour(n int) int {
	if n >= 7 && n <= noonHour {
		return n
	}
	return n + noonHour
}
