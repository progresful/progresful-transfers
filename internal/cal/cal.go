// Package cal has world-calendar helpers. World time is whole minutes from Monday 00:00.
package cal

import (
	"fmt"

	"highwaycoins/internal/api"
)

const (
	Hour = 60
	Day  = 24 * Hour
	Week = 7 * Day

	saturday = 5
	sunday   = 6
	// maxScanDays bounds searches for the next/previous opening.
	maxScanDays = 10
)

var dayNames = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// DayOf is the day index since the first Monday (0 = first Monday, 7 = the next Monday).
func DayOf(t int) int { return t / Day }

// MinuteOfDay is the minute within the day, 0..1439.
func MinuteOfDay(t int) int { return t % Day }

// Weekday is 0 (Monday) .. 6 (Sunday) for a day index.
func Weekday(day int) int { return ((day % 7) + 7) % 7 }

// IsBusinessDay reports whether the day index is Monday to Friday.
func IsBusinessDay(day int) bool { return Weekday(day) < saturday }

// At builds a world minute from a day index and a minute of the day.
func At(day, minuteOfDay int) int { return day*Day + minuteOfDay }

// NextBusinessDay returns the first business day strictly after day.
func NextBusinessDay(day int) int {
	for d := day + 1; ; d++ {
		if IsBusinessDay(d) {
			return d
		}
	}
}

// Format renders a world minute as "Tue 13:05" ("Mon+ 09:00" in the second week).
func Format(t int) string {
	day := DayOf(t)
	suffix := ""
	if day >= 7 {
		suffix = "+"
	}
	return fmt.Sprintf("%s%s %02d:%02d", dayNames[Weekday(day)], suffix, MinuteOfDay(t)/Hour, t%Hour)
}

// HoursOn returns the opening interval (minutes of the day) of a site on a day index.
func HoursOn(h *api.OpeningHours, day int) (opens, closes int, ok bool) {
	return hoursOn(h, day)
}

func hoursOn(h *api.OpeningHours, day int) (opens, closes int, ok bool) {
	wd := Weekday(day)
	switch {
	case wd == sunday:
		return 0, 0, false
	case wd == saturday:
		if h.Saturday == nil {
			return 0, 0, false
		}
		return h.Saturday.Opens, h.Saturday.Closes, h.Saturday.Closes > h.Saturday.Opens
	}
	for _, d := range h.Weekdays {
		if d == wd {
			return h.Opens, h.Closes, h.Closes > h.Opens
		}
	}
	return 0, 0, false
}

// IsOpen reports whether a site with the given stated hours is open at minute t.
// A nil hours value (a private address) counts as open.
func IsOpen(h *api.OpeningHours, t int) bool {
	if h == nil {
		return true
	}
	opens, closes, ok := hoursOn(h, DayOf(t))
	m := MinuteOfDay(t)
	return ok && m >= opens && m <= closes
}

// NextOpen returns the first minute >= t at which the site is open, or -1 if none soon.
func NextOpen(h *api.OpeningHours, t int) int {
	if h == nil {
		return t
	}
	for day := DayOf(t); day < DayOf(t)+maxScanDays; day++ {
		opens, closes, ok := hoursOn(h, day)
		if !ok {
			continue
		}
		start := max(t, At(day, opens))
		if start <= At(day, closes) {
			return start
		}
	}
	return -1
}

// LastOpen returns the last minute <= t at which the site is open, or -1 if none recently.
func LastOpen(h *api.OpeningHours, t int) int {
	if h == nil {
		return t
	}
	for day := DayOf(t); day >= 0 && day > DayOf(t)-maxScanDays; day-- {
		opens, closes, ok := hoursOn(h, day)
		if !ok {
			continue
		}
		end := min(t, At(day, closes))
		if end >= At(day, opens) {
			return end
		}
	}
	return -1
}
