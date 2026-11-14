package notes

import (
	"math/rand"
	"strings"
	"testing"
)

func TestParseCorrections(t *testing.T) {
	runCases(t, []parseCase{
		{"Noudettavissa aikaisintaan huomenna klo 12.00. Korjaus: klo 11.00", at(tue, 14, 4), facts(notBefore(at(wed, 11, 0)))},
		{"Ostaja maksaa ensin, auton saa huomenna klo 10.30 alkaen, eikun klo 11.30. Terveisin myynti.", at(mon, 18, 7), facts(notBefore(at(tue, 11, 30)))},
		{"Ostaja maksaa ensin, auton saa torstaina klo 12.30 alkaen, ei kun klo 11.30", at(tue, 10, 20), facts(notBefore(at(thu, 11, 30)))},
		{"Haettava ennen klo 16, ei kun klo 17. Kiitos taas", at(mon, 13, 52), facts(pickup(None, hm(17, 0)))},
		{"kohde sulkee klo 15.00 (ei klo 14.00). kiitos etukäteen", at(thu, 15, 33), facts(delivery(None, hm(15, 0)))},
		{"Auto valmis vasta huomenna klo 11, ei klo 10. Kohde auki arkisin klo 8 - 20.", at(tue, 17, 42), facts(notBefore(at(wed, 11, 0)), delivery(hm(8, 0), hm(20, 0)))},
		{"Huolto kesken, valmis huomenna klo 15.00, ei kun klo 14.00.", at(mon, 17, 20), facts(notBefore(at(tue, 14, 0)))},
		{"not ready before 14:30, sorry, i mean 15:30", at(mon, 13, 6), facts(notBefore(at(mon, 15, 30)))},
		{"hello. released after payment, not before tomorrow 11:00 (not 10:00)", at(mon, 16, 10), facts(notBefore(at(tue, 11, 0)))},
		{"Hi there. In the workshop, ready tomorrow 11:30 (not 12:30).", at(tue, 15, 57), facts(notBefore(at(wed, 11, 30)))},
		{"perillä liike kiinni klo 14 jälkeen, ei kun klo 15. hyvää päivänjatkoa", at(fri, 12, 17), facts(delivery(None, hm(15, 0)))},
		{"Moikka. Huomenna klo 8 valmis. Perillä ei ketään klo 17 jälkeen, eikun klo 16", at(mon, 15, 45), facts(notBefore(at(tue, 8, 0)), delivery(None, hm(16, 0)))},
		{"At destination tomorrow 12:00 latest. Correction: 13:00. Collect before 20:00.", at(mon, 12, 52), facts(deadline(at(tue, 13, 0)), pickup(None, hm(20, 0)))},
		{"Toimitus perille viim. klo 20. Korjaus: klo 19! Soita asiakkaalle tuntia ennen perillä oloa", at(thu, 13, 52), facts(deadline(at(thu, 19, 0)))},
		{"Oltava perillä viimeistään huomenna puoli kolmelta, ei puoli kahdelta", at(mon, 9, 50), facts(deadline(at(tue, 14, 30)))},
		{"Nouto ennen klo 19, ei klo 20. Oltava perillä viimeistään ma klo 14", at(thu, 17, 59), facts(pickup(None, hm(19, 0)), deadline(at(nextMon, 14, 0)))},
		// A correction of the day alone keeps the time.
		{"Vastaanottava liike auki arkisin 7.00 - 20.00. Auto valmis vasta huomenna klo 14.00, eikun keskiviikkona.", at(mon, 14, 7), facts(notBefore(at(wed, 14, 0)), delivery(hm(7, 0), hm(20, 0)))},
		{"auto valmiina, haettavissa heti. toimitus viimeistään sunnuntaina 18.00. korjaus: maanantaina.", at(thu, 12, 7), facts(deadline(at(nextMon, 18, 0)))},
		{"Must arrive by Sunday 20:00. Correction: Monday.", at(thu, 12, 0), facts(deadline(at(nextMon, 20, 0)))},
		{"Toimitus perille viim. maanantaina klo 20, ei sunnuntaina.", at(wed, 8, 15), facts(deadline(at(nextMon, 20, 0)))},
		{"Liike kiinni kello 20, haettava ennen sitä. Korjaus: kello 19", at(thu, 15, 26), facts(pickup(None, hm(19, 0)))},
	})
}

func TestParseWordHoursAndWeekdays(t *testing.T) {
	runCases(t, []parseCase{
		{"Toimitusosoite auki 9:stä 20:een. Auto valmis vasta ylihuoemnna kymmeneltä", at(mon, 10, 4), facts(notBefore(at(wed, 10, 0)), delivery(hm(9, 0), hm(20, 0)))},
		{"huomenna puoli kymmeneltä valmis", at(thu, 15, 16), facts(notBefore(at(fri, 9, 30)))},
		{"Valmiina huomenna puoli kahdestatoista alkaen. Rengassarja mukaan.", at(wed, 13, 56), facts(notBefore(at(thu, 11, 30)))},
		{"toimitusosoite auki klo 8:00–21:00. voi noutaa kahdesta alkaen", at(wed, 12, 14), facts(notBefore(at(wed, 14, 0)), delivery(hm(8, 0), hm(21, 0)))},
		{"Kahdeltatoista valmis. Terveisin myynti", at(thu, 10, 41), facts(notBefore(at(thu, 12, 0)))},
		{"Auto valmistuu huomenna puoli yhdeltätosita. Arkisin klo 9–19 nouto onnistuu.", at(mon, 16, 54), facts(notBefore(at(tue, 10, 30)), pickup(hm(9, 0), hm(19, 0)))},
		{"Maksun jälkeen, ei ennen huomenna kahdeltatoista.", at(thu, 13, 44), facts(notBefore(at(fri, 12, 0)))},
		{"luovutetaan kun maksu näkyy, aikaisintaan huomenna puoli yhdeltä.", at(thu, 16, 16), facts(notBefore(at(fri, 12, 30)))},
		{"luovutetaan kun maksu näkyy, aikaisintaan ke. puoli kymmeneltä.", at(mon, 14, 4), facts(notBefore(at(wed, 9, 30)))},
		{"Pesu kesken, haettavissa huomenna kymmenen jälkeen", at(mon, 12, 0), facts(notBefore(at(tue, 10, 0)))},
		{"vastaanotto onnistuu klo 9-21. viimeistään maanantaina yhdeltä perillä", at(thu, 16, 47), facts(deadline(at(nextMon, 13, 0)), delivery(hm(9, 0), hm(21, 0)))},
		{"Kiire, asiakas odottaa. Perillä maanantaina neljään mennessä", at(tue, 9, 39), facts(deadline(at(nextMon, 16, 0)))},
		{"hei vaan. toimitus viimeistään maanantaina kahdelta", at(thu, 11, 23), facts(deadline(at(nextMon, 14, 0)))},
		{"Asiakas tulee hakemaan, perillä viim. ylihuomenna kolmelta. Kiitos", at(mon, 12, 0), facts(deadline(at(wed, 15, 0)))},
		{"Ready from half three", at(mon, 9, 0), facts(notBefore(at(mon, 14, 30)))},
		{"klar i morgon halv elva", at(mon, 9, 0), facts(notBefore(at(tue, 10, 30)))},
		// A weekday is 2 to 6 days after arrival and can roll into the next week.
		{"Perillä ma klo 18 mennessä.", at(tue, 10, 19), facts(deadline(at(nextMon, 18, 0)))},
		{"perillä maanantaina klo 18 mennessä. varaus 263155", at(wed, 18, 39), facts(deadline(at(nextMon, 18, 0)))},
		{"At destination Mon 19:00 latest", at(thu, 12, 43), facts(deadline(at(nextMon, 19, 0)))},
		{"Framme senast mån kl 13. Sommardäcken i bakluckan, följer med.", at(fri, 9, 55), facts(deadline(at(nextMon, 13, 0)))},
		{"vastaanotto arkisin klo 9.00-21.00. oltava perillä viimeistään to klo 17.00. kiitos etukäteen.", at(tue, 10, 57), facts(deadline(at(thu, 17, 0)), delivery(hm(9, 0), hm(21, 0)))},
		{"Voi noutaa to. klo 13 alkaen", at(mon, 11, 54), facts(notBefore(at(thu, 13, 0)))},
		{"Not ready before Thurs 2pm", at(mon, 11, 54), facts(notBefore(at(thu, 14, 0)))},
	})
}

func TestParseFlags(t *testing.T) {
	runCases(t, []parseCase{
		{"Manuaali, ei automaatti", at(mon, 9, 52), facts(manual)},
		{"Manual, driver needs a manual licence.", at(mon, 11, 46), facts(manual)},
		{"It's a manual. Br, sales", at(tue, 17, 23), facts(manual)},
		{"Manuaalivaihteinen, tilausnumero 64840", at(wed, 17, 24), facts(manual)},
		{"kulejttajalla oltava oikeus ajaa manuaalia.", at(wed, 15, 34), facts(manual)},
		{"Manuaalviaihteet, kuski tarvii manuaalikortin", at(thu, 11, 9), facts(manual)},
		{"Ei automaatti vaan manuaali", at(fri, 9, 37), facts(manual)},
		{"manuell, inte automat", at(thu, 17, 56), facts(manual)},
		{"Manuellt växlad.", at(mon, 12, 0), facts(manual)},
		{"Mnaual transmission, no automatic. Br, sales", at(mon, 12, 0), facts(manual)},
		{"huom manuuaali", at(tue, 17, 58), facts(manual)},
		{"poistettu rekisteristä, tarvitaaan siirtokilvet", at(mon, 9, 52), facts(plates)},
		{"hi there. unregistered, needs trade plates. thx", at(wed, 10, 43), facts(plates)},
		{"Vaihtoauto takaisin liikkeelle. Kilvet puuttuu, tuo siitrokilvet. Auto pihalla ruudussa 11, avain tiskiltä. Kiitti", at(wed, 17, 5), facts(plates)},
		{"No plates, bring transfer plates", at(thu, 11, 27), facts(plates)},
		{"ei rekkaeita, koekilvet tarvitaan. soita liikkeeseen ennen kuin tulet.", at(tue, 8, 54), facts(plates)},
		{"Auto ilman kilpiä.", at(fri, 13, 23), facts(plates)},
		{"rekisterikilvet irrotettu! avaimet huollosta", at(fri, 14, 12), facts(plates)},
		{"Moi. Kilvet poistetttu, ota koekilvet.", at(mon, 11, 31), facts(plates)},
		{"inga skyltar på bilen. vår ref 267038", at(thu, 16, 22), facts(plates)},
		{"Plates removed, bring trade plates", at(tue, 12, 0), facts(plates)},
		{"Poistettu rekisteristä, tarvitaan siirtkoilvet!", at(fri, 10, 51), facts(plates)},
		{"Summer tyres on, no winters", at(fri, 12, 50), facts(tyres)},
		{"Huom viela kesarenkaat alla", at(tue, 17, 13), facts(tyres)},
		{"Auto vielä kesräenkailla", at(wed, 11, 1), facts(tyres)},
		{"still on summer tyres.", at(thu, 13, 9), facts(tyres)},
		{"har fortfarande sommardäcck på. referens 357067", at(mon, 9, 4), facts(tyres)},
		{"Kesärenkaat alla, ei talvirenkaita.", at(mon, 12, 0), facts(tyres)},
	})
}

// TestParseOtherPhrasings holds wordings that are not in the practice notes
// but say the same things; they pin how the rules generalise.
func TestParseOtherPhrasings(t *testing.T) {
	runCases(t, []parseCase{
		{"Car won't be ready until 14:00", at(mon, 12, 0), facts(notBefore(at(mon, 14, 0)))},
		{"Saa noutaa aikaisintaan torstaina kello yhdeksän", at(mon, 12, 0), facts(notBefore(at(thu, 9, 0)))},
		{"Handover after payment, earliest tomorrow 11:00", at(mon, 12, 0), facts(notBefore(at(tue, 11, 0)))},
		{"Köparen betalar först, bilen lämnas ut tidigast kl 14.30", at(mon, 12, 0), facts(notBefore(at(mon, 14, 30)))},
		{"ready 14:00, sorry, 15:00", at(mon, 12, 0), facts(notBefore(at(mon, 15, 0)))},
		{"klar kl 14, nej kl 15", at(mon, 12, 0), facts(notBefore(at(mon, 15, 0)))},
		{"Vi stänger kl 15", at(mon, 12, 0), facts(pickup(None, hm(15, 0)))},
		{"Suljemme klo 15", at(mon, 12, 0), facts(pickup(None, hm(15, 0)))},
		{"We open at 10", at(mon, 12, 0), facts(pickup(hm(10, 0), None))},
		{"Ei noutoja klo 15 jälkeen", at(mon, 12, 0), facts(pickup(None, hm(15, 0)))},
		{"No pickups after 3pm", at(mon, 12, 0), facts(pickup(None, hm(15, 0)))},
		{"Pickup by 3pm", at(mon, 12, 0), facts(pickup(None, hm(15, 0)))},
		{"Open 8 to 4", at(mon, 12, 0), facts(pickup(hm(8, 0), hm(16, 0)))},
		{"Liike auki arkisin 8-16 ja lauantaisin 10-14", at(mon, 12, 0), facts(pickup(hm(8, 0), hm(16, 0)))},
		{"Mottagaren stänger kl 16", at(mon, 12, 0), facts(delivery(None, hm(16, 0)))},
		{"Recipient available until 3 p.m. only", at(mon, 12, 0), facts(delivery(None, hm(15, 0)))},
		{"kohde sulkee ei 14 vaan 15", at(mon, 12, 0), facts(delivery(None, hm(15, 0)))},
		{"Deliver by Fri noon", at(mon, 12, 0), facts(deadline(at(fri, 12, 0)))},
		{"Leverans senast fredag kl 12", at(mon, 12, 0), facts(deadline(at(fri, 12, 0)))},
		{"Customer collects Friday 2pm", at(mon, 12, 0), facts(deadline(at(fri, 14, 0)))},
		{"No later than Wed 16:00", at(mon, 12, 0), facts(deadline(at(wed, 16, 0)))},
		{"Not an automatic", at(mon, 12, 0), facts(manual)},
		{"Kilvet irti, ota siirtokilvet", at(mon, 12, 0), facts(plates)},
		{"No winter tyres", at(mon, 12, 0), facts(tyres)},
	})
}

func TestParseHarmless(t *testing.T) {
	texts := []string{
		"",
		"kesärenkaat vanteineen takakontissa. avaimet huollon tiskiltä. kiitos paljon",
		"kesäreenkaat vanteineen takakontissa",
		"Rengassarja mukaan",
		"Extra set of tyres in the boot",
		"Summer tyres in the trunk, go with the car",
		"smmer tyres in the trunk",
		"Sommardäcken i bakluckan, följer med. Hälsningar.",
		"Lisäksi 4 kesärengasta tavaratilassa, kiitos taas",
		"Lisäksi 4 kesärenggasta tavaratilassa.",
		"2 sarjaa renkaita, toinen sarja kontissa. Viitenro 653684. t. myynti",
		"Moikka. 2 srjaa renkaita, toinen sarja kontissa",
		"4 rngasta vanteilla tavaratilassa.",
		"second set of weels in the back. call before pickup. thx",
		"Auto pihalla ruudussa 21, avain tiskiltä",
		"car in bay 24, keys at the desk",
		"bilen står på plats 19",
		"Call 30 min before you arrive",
		"call 15 min before you arrvie",
		"Soita 20 min ennen",
		"ring 20 min innan",
		"ring kunden en timme innan",
		"Soita asiakkaalle tuntia ennen perillä oloa",
		"call the customer an hour before arrival",
		"Car is ready and waiting. Key box code 7543.",
		"4 rengasta vanteilla tavaratilassa. avain avainboksissa, koodi 9725.",
		"nyckelbox, kod 2591",
		"Tilausnro 879891",
		"Tilusnumero 892527.",
		"Our ref: 451339",
		"meidän viite 93975",
		"114 tkm",
		"huoltokirja ja rekisteriote autossa",
		"Service book and reg papers in the car",
		"Hei, tässä taas yksi siirto. Kiitos",
		"Myyjä kotona klo 13.45-16.45.",
		"myyjä kotona huomenna klo 10.30 - 13.30, soita n. tuntia ennen saapumista!",
		"Owner at home tomorrow 12.45–3:45pm.",
		"Auto valmis, nouto heti kun sopii.",
		"Ready, collect right away.",
		"Vaihtari tulee takaisin meille.",
		"Trade-in comes back to us",
		"Standard transfer.",
		"Akku heikko, voi tarvita apukäynnistyksen.",
		"Hi there. Nothing special.",
		"Nouto liikkeen pihasta.",
		"Pickup from our yard",
		"Summer tyres included",
		"Hi there, three sets of keys",
		"Car is ready to go",
	}
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			if got := Parse(text, at(tue, 10, 0)); got != empty() {
				t.Errorf("Parse(%q) = %+v, want nothing", text, got)
			}
		})
	}
}

// TestParseNeverPanics feeds broken, truncated and scrambled notes to the
// reader without its safety net.
func TestParseNeverPanics(t *testing.T) {
	seeds := []string{
		"", " ", "....", ",,,", "klo", "klo klo", "ei", "ei kun", "eikun", "12:", ":12", "9:stä", "9:", "-", "–-–", "ma-", "ma-pe",
		"puoli", "half past", "not", "(((", ")", "a.m.", "24:00", "25:99", "klo 2460", "99999999999999999999999", "0", "00.00",
		"🚗 klo 14 ⏰", "Ä", "ÅÄÖ åäö", "to to to", "ei ei ei klo", "korjaus:", "korjaus: korjaus: 12", "i mean", "8-", "-8", "8--16",
		"the day after tomorrow", "after tomorrow", "pm", "12 pm am", "kymmeneltä puoli", strings.Repeat("klo 12 ", 500),
		strings.Repeat("ä", 5000), strings.Repeat("9", 5000), strings.Repeat("ei kun klo 9:stä 20:een, ", 300),
		"Vastaanottaja paikalla vain puoli viiteen saakka / Mottagning till kl 16.30. Liike kiinni kello 20, haettava ennen sitä",
		"hello. released after payment, not before tomorrow 11:00 (not 10:00)",
		"Latest arrival the day after tomorrow 8pm. Nobody here after 4:30pm. Thank you",
	}
	rng := rand.New(rand.NewSource(1))
	const alphabet = "abcdefghijklmnopqrstuvwxyzäöå0123456789 .,:;-–/()!?'\n"
	letters := []rune(alphabet)
	for _, seed := range seeds {
		parse(seed, 0)
		parse(seed, -5000)
		runes := []rune(seed)
		for cut := 0; cut <= len(runes) && cut < 200; cut++ {
			parse(string(runes[:cut]), at(fri, 23, 59))
			parse(string(runes[cut:]), at(mon, 0, 0))
		}
		for round := 0; round < 200 && len(runes) > 0; round++ {
			mutated := append([]rune(nil), runes...)
			for k := 0; k < 3; k++ {
				mutated[rng.Intn(len(mutated))] = letters[rng.Intn(len(letters))]
			}
			parse(string(mutated), rng.Intn(20000))
		}
	}
	for round := 0; round < 2000; round++ {
		noise := make([]rune, rng.Intn(60))
		for i := range noise {
			noise[i] = letters[rng.Intn(len(letters))]
		}
		parse(string(noise), rng.Intn(20000))
	}
}

func TestParseIsDeterministic(t *testing.T) {
	text := "Ostaja maksaa ensin, auton saa huomenna klo 10.30 alkaen, eikun klo 11.30. Kohde auki arkisin 8-16."
	first := Parse(text, at(mon, 18, 7))
	for i := 0; i < 50; i++ {
		if got := Parse(text, at(mon, 18, 7)); got != first {
			t.Fatalf("run %d: got %+v, first %+v", i, got, first)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	texts := []string{
		"Rengassarja mukaan. Kiitos",
		"Ostaja maksaa ensin, auton saa huomenna klo 10.30 alkaen, eikun klo 11.30. Terveisin myynti.",
		"Vastaanottaja paikalla vain puoli viiteen saakka / Mottagning till kl 16.30. Liike kiinni kello 20, haettava ennen sitä",
		"poistettu rekisteristä, tarvitaaan siirtokilvet",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Parse(texts[i%len(texts)], 600)
	}
}
