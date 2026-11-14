// Package notes reads free-text dealer notes (Finnish, Swedish, English, with
// typos, abbreviations and corrections) and extracts the facts the simulated
// world acts on: when a car can really be collected, the real opening hours of
// the pickup and delivery sites, a tighter deadline, and what the driver must
// bring or be allowed to drive.
//
// The reader is deterministic and pure: no I/O, no clock, no global state that
// changes after package initialisation.
//
// How to use the result:
//   - NotBefore is the time in the note. Plan the pickup from the later of it
//     and the stated ready time; the site's hours still apply.
//   - Opening hours that name no site ("Liike auki", "we close at", "oppet")
//     are those of the pickup site. Hours with "kohde", "perilla",
//     "vastaanotto", "receiving", "destination" or "mottagning" are those of
//     the delivery site. Override the stated opens/closes with each value that
//     is not None; a lone "collect before 16" sets only the closing time.
//   - Deadline is the time in the note, which can be later than the order's
//     own deadline: the earlier of the two counts.
//   - A note often restates what the request already says; such values are
//     returned too and change nothing.
//   - In a two-leg trade-in the note is about the first leg.
package notes

// None marks an absent minute value.
const None = -1

const (
	minutesPerDay = 1440
	daysPerWeek   = 7
)

// Facts is what a note changes about a car, relative to what the price request states.
type Facts struct {
	NotBefore      int  // absolute world minute before which the car cannot be collected (ready time, or handover after payment); None if the note sets none
	PickupOpens    int  // minute of day (0..1440) the PICKUP site really opens; None if not stated
	PickupCloses   int  // minute of day the PICKUP site really closes / latest collection time of day; None if not stated
	DeliveryOpens  int  // minute of day the DELIVERY site really starts receiving; None if not stated
	DeliveryCloses int  // minute of day the DELIVERY site stops receiving; None if not stated
	Deadline       int  // absolute world minute by which the car must be at the destination; None if not stated
	Manual         bool // car has a manual gearbox: driver needs a manual licence
	Plates         bool // car has no plates: transfer/trade plates must be fetched
	SummerTyres    bool // car is on summer tyres
}

// empty returns Facts with every time field absent and every flag false.
func empty() Facts {
	return Facts{
		NotBefore:      None,
		PickupOpens:    None,
		PickupCloses:   None,
		DeliveryOpens:  None,
		DeliveryCloses: None,
		Deadline:       None,
	}
}

// Parse reads one note. arrivedAt is the world minute the price request arrived.
// It never panics: if the reader fails on an input, the note states nothing.
func Parse(text string, arrivedAt int) (facts Facts) {
	defer func() {
		if r := recover(); r != nil {
			facts = empty()
		}
	}()
	return parse(text, arrivedAt)
}

// parse is Parse without the safety net, so that tests see a failure.
func parse(text string, arrivedAt int) Facts {
	if text == "" {
		return empty()
	}
	segs := tokenize(text)
	facts := assemble(extract(segs), arrivedAt)
	facts.Manual, facts.Plates, facts.SummerTyres = flags(segs)
	return facts
}
