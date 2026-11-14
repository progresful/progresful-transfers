package notes

import "testing"

func TestOsaDistance(t *testing.T) {
	cases := []struct {
		a, b  string
		limit int
		want  int
	}{
		{"fredag", "fredag", 2, 0},
		{"frdeag", "fredag", 2, 1},           // swapped letters
		{"ylihuoemnna", "ylihuomenna", 2, 1}, // swapped letters
		{"tarvitaaan", "tarvitaan", 2, 1},    // doubled letter
		{"receivng", "receiving", 2, 1},      // dropped letter
		{"beatlning", "betalning", 2, 1},
		{"kiitti", "kiinni", 1, 2}, // over the limit: limit+1
		{"", "abc", 3, 3},
		{"abc", "", 1, 2},
		{"auki", "auto", 2, 2},
	}
	for _, c := range cases {
		if got := osaDistance(c.a, c.b, c.limit); got != c.want {
			t.Errorf("osaDistance(%q, %q, %d) = %d, want %d", c.a, c.b, c.limit, got, c.want)
		}
	}
}

func TestLookupReadsTypos(t *testing.T) {
	cases := []struct {
		word string
		want feat
	}{
		{"valmis", fReady},
		{"vamlis", fReady},
		{"vaalmis", fReady},
		{"aukiolajat", fOpen},
		{"vstaanottava", fDest},
		{"mottagnign", fDest},
		{"frmame", fArrive},
		{"preilla", fDest | fArrive},
		{"viimeisstaan", fLatest},
		{"siitrokilvet", fPlateNeed},
		{"manuaalviaihteet", fManual},
		{"kesraenkailla", fSummer},
		{"toorsdag", fWeekday},
		{"tomororw", fDay},
		{"colelct", fPickup},
	}
	for _, c := range cases {
		if got, _ := lookup(c.word); got&c.want != c.want {
			t.Errorf("lookup(%q) = %b, want bits %b", c.word, got, c.want)
		}
	}
}

func TestLookupKeepsLookAlikesApart(t *testing.T) {
	cases := []struct {
		word string
		not  feat
	}{
		{"rekisteriote", fUnreg},  // a document, not "rekisterista"
		{"rekisteritoe", fUnreg},  // the same, with a typo
		{"kiitti", fClose},        // thanks, not "kiinni"
		{"auto", fOpen},           // car, not "auki"
		{"viite", fHour},          // reference, not "viiteen"
		{"there", fHour},          // "hi there", not "three"
		{"toimistosta", fArrive},  // from the office, not "toimitus"
		{"kesarengasta", fSummer}, // "4 kesarengasta": cargo
		{"waiting", fCorr},        // not "wait"
	}
	for _, c := range cases {
		if got, _ := lookup(c.word); got&c.not != 0 {
			t.Errorf("lookup(%q) = %b, must not have bits %b", c.word, got, c.not)
		}
	}
}

func TestTokenizeSplitsClauses(t *testing.T) {
	cases := []struct {
		text      string
		segments  int
		sentences int
	}{
		{"", 0, 0},
		{"Liike auki 9.00-16.30.", 1, 1},
		{"perillä viim. to. klo 12.30 mennessä", 1, 1}, // abbreviations and "12.30" do not end the sentence
		{"klo 15. Auki vain klo 18 asti", 2, 2},
		{"Nouto ennen klo 15.30 / Collect before 15:30.", 2, 2},
		{"klo 10.30 alkaen, eikun klo 11.30", 2, 1},
		{"sulkee klo 15.00 (ei klo 14.00)", 2, 1},
	}
	for _, c := range cases {
		segs := tokenize(c.text)
		sentences := 0
		if len(segs) > 0 {
			sentences = segs[len(segs)-1].sentence + 1
		}
		if len(segs) != c.segments || sentences != c.sentences {
			t.Errorf("tokenize(%q): %d segments in %d sentences, want %d in %d", c.text, len(segs), sentences, c.segments, c.sentences)
		}
	}
}

func TestScanTimes(t *testing.T) {
	cases := []struct {
		text       string
		start, end int
		day, wd    int
	}{
		{"klo 15.30", hm(15, 30), None, None, None},
		{"huomenna klo 9", hm(9, 0), None, 1, None},
		{"ylihuomenna kymmeneltä", hm(10, 0), None, 2, None},
		{"the day after tomorrow 8pm", hm(20, 0), None, 2, None},
		{"friday 2:30pm", hm(14, 30), None, None, 4},
		{"kello 1400 jälkeen", hm(14, 0), None, None, None},
		{"puoli viiteen saakka", hm(16, 30), None, None, None},
		{"half past two", hm(14, 30), None, None, None},
		{"klockan två", hm(14, 0), None, None, None},
		{"ennen kolmea", hm(15, 0), None, None, None},
		{"8-15.30", hm(8, 0), hm(15, 30), None, None},
		{"9:stä 20:een", hm(9, 0), hm(20, 0), None, None},
		{"10 am-4:30 pm", hm(10, 0), hm(16, 30), None, None},
		{"12.45–3:45pm", hm(12, 45), hm(15, 45), None, None},
		{"ma–pe 9-17", hm(9, 0), hm(17, 0), None, None},
		{"9 to 5", hm(9, 0), hm(17, 0), None, None},
		{"12 pm", hm(12, 0), None, None, None},
		{"12 am", 0, None, None, None},
		{"by 5pm tomorrow", hm(17, 0), None, 1, None},
	}
	for _, c := range cases {
		segs := tokenize(c.text)
		if len(segs) != 1 {
			t.Errorf("%q: %d segments, want 1", c.text, len(segs))
			continue
		}
		exprs, _, _ := scanTimes(&segs[0])
		if len(exprs) != 1 {
			t.Errorf("%q: %d time expressions %+v, want 1", c.text, len(exprs), exprs)
			continue
		}
		want := timeExpr{day: c.day, wd: c.wd, start: c.start, end: c.end}
		if exprs[0] != want {
			t.Errorf("%q: got %+v, want %+v", c.text, exprs[0], want)
		}
	}
}

func TestScanTimesIgnoresOtherNumbers(t *testing.T) {
	texts := []string{
		"auto pihalla ruudussa 21", "car in bay 24", "koodi 1019", "key box code 4688", "tilausnro 879891",
		"call 15 min before you arrive", "114 tkm", "2 sarjaa renkaita", "4 rengasta vanteilla", "tässä taas yksi siirto",
		"car is ready to go", "two sets of tyres", "bilen står på plats 19",
	}
	for _, text := range texts {
		for _, seg := range tokenize(text) {
			if exprs, _, _ := scanTimes(&seg); len(exprs) != 0 {
				t.Errorf("%q: read %+v, want no time", text, exprs)
			}
		}
	}
}

func TestAbsoluteCountsDaysFromArrival(t *testing.T) {
	clock := func(day, wd int) timeExpr { return timeExpr{day: day, wd: wd, start: hm(12, 0), end: None} }
	cases := []struct {
		name    string
		expr    timeExpr
		arrived int
		want    int
	}{
		{"no day is the arrival day", clock(None, None), at(wed, 17, 0), at(wed, 12, 0)},
		{"tomorrow", clock(1, None), at(fri, 9, 0), at(fri+1, 12, 0)},
		{"the day after tomorrow", clock(2, None), at(thu, 9, 0), at(thu+2, 12, 0)},
		{"thursday from monday", clock(None, thu), at(mon, 9, 0), at(thu, 12, 0)},
		{"monday from thursday rolls into the next week", clock(None, mon), at(thu, 9, 0), at(nextMon, 12, 0)},
		{"monday from tuesday is six days later", clock(None, mon), at(tue, 9, 0), at(nextMon, 12, 0)},
		{"the arrival weekday is a week later", clock(None, wed), at(wed, 9, 0), at(wed+daysPerWeek, 12, 0)},
		{"second week", clock(None, wed), at(nextMon, 9, 0), at(nextMon+2, 12, 0)},
	}
	for _, c := range cases {
		if got := absolute(c.expr, c.arrived); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
