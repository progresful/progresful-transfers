package cal

import (
	"testing"

	"highwaycoins/internal/api"
)

func hours() *api.OpeningHours {
	return &api.OpeningHours{Opens: 8 * Hour, Closes: 18 * Hour, Weekdays: []int{0, 1, 2, 3, 4}, Saturday: &api.Saturday{Opens: 9 * Hour, Closes: 14 * Hour}}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		at   int
		want string
	}{
		{0, "Mon 00:00"},
		{At(1, 13*Hour+5), "Tue 13:05"},
		{At(6, 23*Hour+59), "Sun 23:59"},
		{At(7, 9*Hour), "Mon+ 09:00"},
	}
	for _, tc := range tests {
		if got := Format(tc.at); got != tc.want {
			t.Errorf("Format(%d) = %q, want %q", tc.at, got, tc.want)
		}
	}
}

func TestBusinessDays(t *testing.T) {
	if !IsBusinessDay(4) || IsBusinessDay(5) || IsBusinessDay(6) || !IsBusinessDay(7) {
		t.Fatal("Monday to Friday are business days, in the second week too")
	}
	tests := []struct{ day, want int }{{0, 1}, {3, 4}, {4, 7}, {5, 7}, {6, 7}}
	for _, tc := range tests {
		if got := NextBusinessDay(tc.day); got != tc.want {
			t.Errorf("NextBusinessDay(%d) = %d, want %d", tc.day, got, tc.want)
		}
	}
}

func TestOpenings(t *testing.T) {
	h := hours()
	tests := []struct {
		name                     string
		at                       int
		open                     bool
		wantNextOpen, wantLastOp int
	}{
		{"weekday inside hours", At(1, 12*Hour), true, At(1, 12*Hour), At(1, 12*Hour)},
		{"weekday before opening", At(1, 6*Hour), false, At(1, 8*Hour), At(0, 18*Hour)},
		{"weekday after closing", At(1, 19*Hour), false, At(2, 8*Hour), At(1, 18*Hour)},
		{"friday evening rolls to saturday's short hours", At(4, 20*Hour), false, At(5, 9*Hour), At(4, 18*Hour)},
		{"saturday afternoon rolls over sunday to monday", At(5, 15*Hour), false, At(7, 8*Hour), At(5, 14*Hour)},
		{"sunday is closed", At(6, 12*Hour), false, At(7, 8*Hour), At(5, 14*Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOpen(h, tc.at); got != tc.open {
				t.Errorf("IsOpen = %v, want %v", got, tc.open)
			}
			if got := NextOpen(h, tc.at); got != tc.wantNextOpen {
				t.Errorf("NextOpen = %s, want %s", Format(got), Format(tc.wantNextOpen))
			}
			if got := LastOpen(h, tc.at); got != tc.wantLastOp {
				t.Errorf("LastOpen = %s, want %s", Format(got), Format(tc.wantLastOp))
			}
		})
	}
}

func TestPrivateAddressIsAlwaysOpen(t *testing.T) {
	at := At(6, 3*Hour)
	if !IsOpen(nil, at) || NextOpen(nil, at) != at || LastOpen(nil, at) != at {
		t.Error("a place without hours (a private address) counts as open at any minute")
	}
}

func TestClosedSaturday(t *testing.T) {
	h := hours()
	h.Saturday = nil
	if got := NextOpen(h, At(4, 19*Hour)); got != At(7, 8*Hour) {
		t.Errorf("NextOpen over a closed Saturday = %s, want Mon+ 08:00", Format(got))
	}
}
