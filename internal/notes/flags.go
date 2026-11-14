package notes

import "strings"

// flags reads the three yes/no facts: manual gearbox, missing plates, summer tyres.
func flags(segs []segment) (manual, plates, summer bool) {
	for i := range segs {
		toks := segs[i].toks
		manual = manual || manualIn(toks)
		plates = plates || platesIn(toks)
		summer = summer || summerIn(toks)
	}
	return manual, plates, summer
}

func wordAt(toks []token, i int) *token {
	if i < 0 || i >= len(toks) || toks[i].kind != kWord {
		return nil
	}
	return &toks[i]
}

// negatedAt reports whether the word at i is denied: "ei manuaali", "not a manual".
func negatedAt(toks []token, i int) bool {
	p := wordAt(toks, i-1)
	if p != nil && (p.text == "a" || p.text == "an") {
		p = wordAt(toks, i-2)
	}
	return p != nil && p.f&fNeg != 0
}

// manualIn: "Manuaali, ei automaatti", "It's a manual", "no automatic".
func manualIn(toks []token) bool {
	for i := range toks {
		t := wordAt(toks, i)
		if t == nil {
			continue
		}
		if t.f&fManual != 0 && !negatedAt(toks, i) {
			if next := wordAt(toks, i+1); t.text == "manual" && next != nil && next.text == "in" {
				continue // the handbook, in the glovebox
			}
			return true
		}
		if t.f&fAuto != 0 && negatedAt(toks, i) {
			return true
		}
	}
	return false
}

// platesIn: transfer plates are asked for, the car is unregistered, or its
// plates are said to be missing.
func platesIn(toks []token) bool {
	var seen feat
	for i := range toks {
		t := wordAt(toks, i)
		if t == nil {
			continue
		}
		seen |= t.f
		if p := wordAt(toks, i-1); t.f&fPlate != 0 && p != nil && (p.text == "trade" || p.text == "transfer" || p.text == "temporary") {
			return true
		}
	}
	if seen&(fPlateNeed|fUnreg) != 0 {
		return true
	}
	return seen&fPlate != 0 && seen&(fMissing|fNeg) != 0
}

// summerIn: the car stands on summer tyres. Summer tyres that travel in the
// boot are cargo, not the flag. Winter tyres that are denied anywhere in the
// clause ("Talvirenkaita ei vielä vaihdettu") mean the same as summer tyres on.
func summerIn(toks []token) bool {
	onSummers, cargo, winter, denied, summerWord := false, false, false, false, false
	for i := range toks {
		t := wordAt(toks, i)
		if t == nil {
			continue
		}
		if t.f&(fNeg|fMissing) != 0 {
			denied = true
		}
		switch {
		case t.f&fWinter != 0:
			winter = true
			if p := wordAt(toks, i-1); p != nil && p.f&(fNeg|fMissing) != 0 {
				return true // "no winters", "ei talvirenkaita"
			}
		case t.f&fSummer != 0:
			summerWord = true
			next := wordAt(toks, i+1)
			if t.text != "summer" || (next != nil && next.f&fTyre != 0) {
				onSummers = true
			}
		case t.f&fCargo != 0, len(t.text) > 5 && strings.HasSuffix(t.text, "ssa"):
			cargo = true
		}
	}
	if cargo {
		return false
	}
	return onSummers || (winter && denied && !summerWord)
}
