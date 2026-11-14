package main

import (
	"highwaycoins/internal/api"
	"highwaycoins/internal/notes"
)

// verdict of one field of one leg.
type verdict uint8

const (
	ok       verdict = iota
	benign           // the note restates what the request already says
	missed           // truth has an effect, the reader found none (false negative)
	wrong            // truth has an effect, the reader found another value
	spurious         // truth has no effect, the reader invented one (false positive)
)

var fieldNames = []string{"notBefore", "pickupHours", "deliveryHours", "deadline", "manual", "plates", "summerTyres"}

// expected is the Facts a perfect reader returns for this leg.
func expected(leg api.Leg, t api.LegTruth) notes.Facts {
	want := notes.Parse("", 0)
	switch {
	case t.ReleasedAt != nil:
		want.NotBefore = *t.ReleasedAt
	case t.ReadyAt != leg.Pickup.Window.From:
		want.NotBefore = t.ReadyAt
	}
	if s, r := leg.Pickup.Hours, t.PickupHours; s != nil && r != nil {
		if r.Opens != s.Opens {
			want.PickupOpens = r.Opens
		}
		if r.Closes != s.Closes {
			want.PickupCloses = r.Closes
		}
	}
	if s, r := leg.Delivery.Hours, t.DeliveryHours; s != nil && r != nil {
		if r.Opens != s.Opens {
			want.DeliveryOpens = r.Opens
		}
		if r.Closes != s.Closes {
			want.DeliveryCloses = r.Closes
		}
	}
	if t.NoteDeadline != nil {
		want.Deadline = *t.NoteDeadline
	}
	want.Manual, want.Plates, want.SummerTyres = t.ManualGearbox, t.NeedsPlates, t.SummerTyres
	return want
}

// judge compares what was read with the truth, field by field, in the order of fieldNames.
func judge(leg api.Leg, t api.LegTruth, optDeadline int, got notes.Facts) [7]verdict {
	want := expected(leg, t)
	return [7]verdict{
		judgeNotBefore(want.NotBefore, got.NotBefore, leg.Pickup.Window.From),
		judgeHours(leg.Pickup.Hours, t.PickupHours, got.PickupOpens, got.PickupCloses),
		judgeHours(leg.Delivery.Hours, t.DeliveryHours, got.DeliveryOpens, got.DeliveryCloses),
		judgeDeadline(want.Deadline, got.Deadline, optDeadline),
		judgeFlag(want.Manual, got.Manual),
		judgeFlag(want.Plates, got.Plates),
		judgeFlag(want.SummerTyres, got.SummerTyres),
	}
}

// judgeNotBefore: a read time at or before the stated ready time changes
// nothing (the note restates the request), so it counts as benign.
func judgeNotBefore(want, got, stated int) verdict {
	switch {
	case want == notes.None && got == notes.None:
		return ok
	case want == notes.None && got <= stated:
		return benign
	case want == notes.None:
		return spurious
	case got == notes.None:
		return missed
	case got != want:
		return wrong
	}
	return ok
}

// judgeHours: the stated hours, overridden by what was read, must be the true hours.
func judgeHours(stated, real *api.OpeningHours, gotOpens, gotCloses int) verdict {
	if stated == nil || real == nil {
		return ok // private address: no hours to compare
	}
	opens, closes := stated.Opens, stated.Closes
	if gotOpens != notes.None {
		opens = gotOpens
	}
	if gotCloses != notes.None {
		closes = gotCloses
	}
	changed := real.Opens != stated.Opens || real.Closes != stated.Closes
	read := gotOpens != notes.None || gotCloses != notes.None
	switch {
	case opens == real.Opens && closes == real.Closes && read && !changed:
		return benign
	case opens == real.Opens && closes == real.Closes:
		return ok
	case !changed:
		return spurious
	case !read:
		return missed
	}
	return wrong
}

// judgeDeadline: without a note deadline in the truth, a read deadline that is
// not before the chosen option's deadline restates the request.
func judgeDeadline(want, got, optDeadline int) verdict {
	switch {
	case want == notes.None && got == notes.None:
		return ok
	case want == notes.None && optDeadline != notes.None && got >= optDeadline:
		return benign
	case want == notes.None:
		return spurious
	case got == notes.None:
		return missed
	case got != want:
		return wrong
	}
	return ok
}

func judgeFlag(want, got bool) verdict {
	switch {
	case want == got:
		return ok
	case want:
		return missed
	}
	return spurious
}
