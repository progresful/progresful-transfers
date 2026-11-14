// Command noteseval measures the note reader against the truth in the reveal
// of finished practice worlds.
//
//	noteseval [-q] [-blind] [-restated] <run dir>...
//	noteseval -stdin < notes.txt
//
// Each run dir holds a reveal.json. For every order that was won, the reveal
// says what was true of each car; the command compares that with what
// notes.Parse reads from the request's note.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/notes"
)

const (
	minutesPerDay = 1440
	outcomeWon    = "won"
	stdinArrival  = 12 * 60 // -stdin reads every note as arriving on Monday 12:00
	maxLineBytes  = 1 << 20
)

type tally struct {
	legs, correct, noted, notedCorrect int
	restated                           int
	missed, wrong, spurious            map[string]int
	lines, benign                      []string
}

func newTally() *tally {
	return &tally{missed: map[string]int{}, wrong: map[string]int{}, spurious: map[string]int{}}
}

func main() {
	quiet := flag.Bool("q", false, "do not print each mismatch")
	blind := flag.Bool("blind", false, "baseline: read no note at all")
	restated := flag.Bool("restated", false, "also print each field that was read but only restates the request")
	stdin := flag.Bool("stdin", false, "read one note per line from standard input and print what each states (arrival Monday 12:00)")
	flag.Parse()
	if *stdin {
		readLines()
		return
	}
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: noteseval [-q] [-blind] [-restated] <run dir>...")
		os.Exit(2)
	}
	single, tradeLeg0, tradeLeg1, tradeLeg1AsNoted := newTally(), newTally(), newTally(), newTally()
	requests, notesRead := 0, 0
	for _, dir := range flag.Args() {
		rev, err := loadReveal(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", dir, err)
			continue
		}
		truth := make(map[string][]api.LegTruth, len(rev.Orders))
		for _, o := range rev.Orders {
			truth[o.OrderID] = o.Legs
		}
		for _, rr := range rev.Requests {
			requests++
			text := ""
			if rr.Request.Note != nil {
				text = rr.Request.Note.Text
				notesRead++
			}
			facts := notes.Parse(text, rr.Request.ArrivedAt)
			if *blind {
				facts = notes.Parse("", 0)
			}
			legs, won := truth[rr.Request.ID]
			if rr.Outcome != outcomeWon || !won {
				continue
			}
			deadline := optionDeadline(rr)
			for i := range rr.Request.Legs {
				if i >= len(legs) {
					break
				}
				leg, t := rr.Request.Legs[i], legs[i]
				switch {
				case len(rr.Request.Legs) == 1:
					single.add(text, rr.Request.ArrivedAt, leg, t, deadline, facts)
				case i == 0:
					tradeLeg0.add(text, rr.Request.ArrivedAt, leg, t, deadline, facts)
				default:
					tradeLeg1.add(text, rr.Request.ArrivedAt, leg, t, deadline, notes.Parse("", 0))
					tradeLeg1AsNoted.add(text, rr.Request.ArrivedAt, leg, t, deadline, facts)
				}
			}
		}
	}
	fmt.Printf("requests %d, with a note %d (every note was read, also of lost requests)\n\n", requests, notesRead)
	single.report("single-leg orders (the score)", *quiet)
	if *restated {
		sort.Strings(single.benign)
		for _, l := range single.benign {
			fmt.Println(l)
		}
		fmt.Println()
	}
	tradeLeg0.report("trade-ins, leg 0 read with the note", *quiet)
	tradeLeg1.report("trade-ins, leg 1 read as if it had no note", *quiet)
	tradeLeg1AsNoted.report("trade-ins, leg 1 read with the note (for comparison)", true)
}

// readLines prints the facts of each note on standard input, for trying
// phrasings by hand.
func readLines() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, maxLineBytes), maxLineBytes)
	for sc.Scan() {
		fmt.Printf("%-60s | %s\n", describe(notes.Parse(sc.Text(), stdinArrival)), sc.Text())
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read notes: %v\n", err)
		os.Exit(1)
	}
}

func loadReveal(dir string) (*api.Reveal, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "reveal.json"))
	if err != nil {
		return nil, fmt.Errorf("read reveal: %w", err)
	}
	var rev api.Reveal
	if err := json.Unmarshal(raw, &rev); err != nil {
		return nil, fmt.Errorf("decode reveal: %w", err)
	}
	return &rev, nil
}

// optionDeadline is the deadline of the option the dealer chose, or -1.
func optionDeadline(rr api.RevealedRequest) int {
	if rr.Option == nil {
		return notes.None
	}
	for _, o := range rr.Request.Options {
		if o.Kind == *rr.Option {
			return o.Deadline
		}
	}
	return notes.None
}

func (t *tally) add(text string, arrivedAt int, leg api.Leg, truth api.LegTruth, optDeadline int, got notes.Facts) {
	t.legs++
	verdicts := judge(leg, truth, optDeadline, got)
	good := true
	var bad []string
	for i, v := range verdicts {
		name := fieldNames[i]
		switch v {
		case benign:
			t.restated++
			t.benign = append(t.benign, fmt.Sprintf("%-14s arr %s | %q | got %s | stated ready %s, option deadline %s",
				name, clock(arrivedAt), text, describe(got), clock(leg.Pickup.Window.From), clock(optDeadline)))
		case missed:
			t.missed[name]++
		case wrong:
			t.wrong[name]++
		case spurious:
			t.spurious[name]++
		}
		if v > benign {
			good = false
			bad = append(bad, name)
		}
	}
	if good {
		t.correct++
	}
	if text == "" {
		return
	}
	t.noted++
	if good {
		t.notedCorrect++
		return
	}
	t.lines = append(t.lines, fmt.Sprintf("%-22s arr %s | %q | want %s | got %s",
		strings.Join(bad, "+"), clock(arrivedAt), text, describe(expected(leg, truth)), describe(got)))
}

func (t *tally) report(title string, quiet bool) {
	fmt.Printf("== %s ==\n", title)
	if t.legs == 0 {
		fmt.Printf("no legs\n\n")
		return
	}
	fmt.Printf("legs %d, fully correct %d (%.2f%%)\n", t.legs, t.correct, share(t.correct, t.legs))
	fmt.Printf("noted legs %d, fully correct %d (%.2f%%), fields that restate the request %d\n",
		t.noted, t.notedCorrect, share(t.notedCorrect, t.noted), t.restated)
	fmt.Printf("%-14s %8s %8s %8s\n", "field", "missed", "wrong", "spurious")
	for _, name := range fieldNames {
		fmt.Printf("%-14s %8d %8d %8d\n", name, t.missed[name], t.wrong[name], t.spurious[name])
	}
	if !quiet {
		sort.Strings(t.lines)
		for _, l := range t.lines {
			fmt.Println(l)
		}
	}
	fmt.Println()
}

func share(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

var dayNames = [...]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}

// clock prints a world minute as "Tu+14:30" ("+" marks the second week).
func clock(m int) string {
	if m < 0 {
		return "-"
	}
	day := m / minutesPerDay
	week := ""
	if day >= len(dayNames) {
		week = "+"
	}
	return fmt.Sprintf("%s%s%02d:%02d", dayNames[day%len(dayNames)], week, m%minutesPerDay/60, m%60)
}

func hhmm(m int) string {
	if m < 0 {
		return "-"
	}
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

func describe(f notes.Facts) string {
	var parts []string
	if f.NotBefore != notes.None {
		parts = append(parts, "notBefore="+clock(f.NotBefore))
	}
	if f.PickupOpens != notes.None || f.PickupCloses != notes.None {
		parts = append(parts, "pickup="+hhmm(f.PickupOpens)+".."+hhmm(f.PickupCloses))
	}
	if f.DeliveryOpens != notes.None || f.DeliveryCloses != notes.None {
		parts = append(parts, "delivery="+hhmm(f.DeliveryOpens)+".."+hhmm(f.DeliveryCloses))
	}
	if f.Deadline != notes.None {
		parts = append(parts, "deadline="+clock(f.Deadline))
	}
	if f.Manual {
		parts = append(parts, "manual")
	}
	if f.Plates {
		parts = append(parts, "plates")
	}
	if f.SummerTyres {
		parts = append(parts, "summerTyres")
	}
	if len(parts) == 0 {
		return "(nothing)"
	}
	return strings.Join(parts, " ")
}
