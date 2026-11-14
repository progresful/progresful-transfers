package notes

import "testing"

// Days of the first week, as world days.
const (
	mon = iota
	tue
	wed
	thu
	fri
	nextMon = 7
)

// at is a world minute: day d at h:m.
func at(d, h, m int) int { return d*minutesPerDay + h*60 + m }

// hm is a minute of day.
func hm(h, m int) int { return h*60 + m }

type opt func(*Facts)

func facts(opts ...opt) Facts {
	f := empty()
	for _, o := range opts {
		o(&f)
	}
	return f
}

func notBefore(m int) opt { return func(f *Facts) { f.NotBefore = m } }
func deadline(m int) opt  { return func(f *Facts) { f.Deadline = m } }
func pickup(opens, closes int) opt {
	return func(f *Facts) { f.PickupOpens, f.PickupCloses = opens, closes }
}
func delivery(opens, closes int) opt {
	return func(f *Facts) { f.DeliveryOpens, f.DeliveryCloses = opens, closes }
}
func manual(f *Facts) { f.Manual = true }
func plates(f *Facts) { f.Plates = true }
func tyres(f *Facts)  { f.SummerTyres = true }

type parseCase struct {
	text    string
	arrived int
	want    Facts
}

func runCases(t *testing.T, cases []parseCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			if got := Parse(c.text, c.arrived); got != c.want {
				t.Errorf("Parse(%q, %d)\n got %+v\nwant %+v", c.text, c.arrived, got, c.want)
			}
		})
	}
}

func TestParseReadyLater(t *testing.T) {
	runCases(t, []parseCase{
		{"Terve. Auto valmis vasta huomenna 12.00", at(mon, 17, 17), facts(notBefore(at(tue, 12, 0)))},
		{"Auto valmis vasta klo 14.00.", at(tue, 10, 55), facts(notBefore(at(tue, 14, 0)))},
		{"Huolto kesken, valmis perjantaina klo 13", at(wed, 10, 42), facts(notBefore(at(fri, 13, 0)))},
		{"earliest pickup friday 2:30pm.", at(mon, 17, 51), facts(notBefore(at(fri, 14, 30)))},
		{"ready for piickup from 10:30. booking ref 134698", at(thu, 8, 59), facts(notBefore(at(thu, 10, 30)))},
		{"voi noutaa klo 13.30 alkaen / ready for pickup from 13:30", at(fri, 10, 29), facts(notBefore(at(fri, 13, 30)))},
		{"Saa hakea vasta huomenna kello 1400 jälkeen. Varaus 865230", at(mon, 12, 7), facts(notBefore(at(tue, 14, 0)))},
		{"Ready tmrw 1pm at the earliest.", at(mon, 15, 7), facts(notBefore(at(tue, 13, 0)))},
		{"earliest pickup tomorrow 1 pm. call asap if any issues", at(mon, 15, 35), facts(notBefore(at(tue, 13, 0)))},
		{"Moikka. Auto valmis vasta huomenna klo 11 / Kan hämtas tidigast i morgon kl 11", at(tue, 10, 37), facts(notBefore(at(wed, 11, 0)))},
		{"Ready for pickup from Thu 12:00. Cheers", at(tue, 11, 11), facts(notBefore(at(thu, 12, 0)))},
		{"not ready before 16:00.", at(tue, 12, 19), facts(notBefore(at(tue, 16, 0)))},
		{"Auto valmistuu huomenna 11:00", at(wed, 10, 16), facts(notBefore(at(thu, 11, 0)))},
		{"In the workshop, ready 11am. Thanks in advance.", at(thu, 9, 37), facts(notBefore(at(thu, 11, 0)))},
		{"Klar först fre kl 14. Hälsningar", at(wed, 12, 38), facts(notBefore(at(fri, 14, 0)))},
		{"Hej. Kan hämtas i morgon kl 10 och framåt", at(thu, 10, 53), facts(notBefore(at(fri, 10, 0)))},
		{"Bilen är klar 13.30", at(fri, 11, 30), facts(notBefore(at(fri, 13, 30)))},
		{"Auto valmis vasta torstaina klo 12", at(mon, 20, 41), facts(notBefore(at(thu, 12, 0)))},
		{"moi. pesu kesken, haettavissa kello 15.30 jälkeen.", at(mon, 12, 0), facts(notBefore(at(mon, 15, 30)))},
		{"Auto vaalmis vasta klo 13.30. Tilausnumero 236452", at(wed, 9, 57), facts(notBefore(at(wed, 13, 30)))},
		{"Ready for picukp from 13:00.", at(tue, 10, 41), facts(notBefore(at(tue, 13, 0)))},
		{"Noudettavissa aikasintaan klo 12.", at(wed, 10, 44), facts(notBefore(at(wed, 12, 0)))},
		{
			"mottagning vardagar kl 9-20. kan hämtas på toorsdag kl 12 och framåt. extra däcksats i bagaget, ska med.",
			at(tue, 10, 18), facts(notBefore(at(thu, 12, 0)), delivery(hm(9, 0), hm(20, 0))),
		},
		{
			"Voi noutaa keskiviikkona klo 14 alkaen. Haettava ennen klo 19",
			at(mon, 17, 32), facts(notBefore(at(wed, 14, 0)), pickup(None, hm(19, 0))),
		},
		{
			"Auto valmis vasta klo 15. Auki vain klo 18 asti",
			at(tue, 13, 5), facts(notBefore(at(tue, 15, 0)), pickup(None, hm(18, 0))),
		},
		{
			"pickup 7:00 - 19:00 only. ready for pickup from 14:00. summer tyres in the trunk, go with the car.",
			at(fri, 12, 26), facts(notBefore(at(fri, 14, 0)), pickup(hm(7, 0), hm(19, 0))),
		},
	})
}

func TestParseHandoverAfterPayment(t *testing.T) {
	runCases(t, []parseCase{
		{"maksun jälkeen, ei ennen klo 12.30. kiitos.", at(mon, 9, 7), facts(notBefore(at(mon, 12, 30)))},
		{"Luovutetaan kun maksu näkyy, aikaisintaan kello 14.30.", at(tue, 10, 45), facts(notBefore(at(tue, 14, 30)))},
		{"ostaja maksaa ensin, auton saa huomenna klo 13.00 alkaen. kiitos", at(wed, 11, 21), facts(notBefore(at(thu, 13, 0)))},
		{"luovutus vasta kun maksettu, aikaisintaan huomenna klo 12. terveisin myynti", at(thu, 10, 40), facts(notBefore(at(fri, 12, 0)))},
		{"Luovutus vasta kun makettu, aikaisintaan huomenna klo 12.30. Asiakkaan vanha auto takaisin meille.", at(thu, 14, 1), facts(notBefore(at(fri, 12, 30)))},
		{"lämnas ut efter betalning, tidigast frdeag kl 15. tack.", at(tue, 17, 40), facts(notBefore(at(fri, 15, 0)))},
		{"Hej. Lämnas ut efter beatlning, tidigast i morgon kl 13.30.", at(tue, 10, 57), facts(notBefore(at(wed, 13, 30)))},
		{"lämnas ut efter betalning, tidigast på fredag kl 11", at(wed, 12, 33), facts(notBefore(at(fri, 11, 0)))},
		{"Hi there. Buyer pays first, pickup from 14:00.", at(fri, 9, 7), facts(notBefore(at(fri, 14, 0)))},
		{"Buyer pays first, pickup from tomorrow 9:30", at(tue, 10, 44), facts(notBefore(at(wed, 9, 30)))},
		{"Payment clears tmrw 12:30, no pickup before that.", at(mon, 11, 0), facts(notBefore(at(tue, 12, 30)))},
		{"Maksun jälkeen, ei ennen huomenna klo 9.", at(thu, 15, 20), facts(notBefore(at(fri, 9, 0)))},
		{
			"Terve. Asiakas maksaa tänään 14.00, luovutus sen jälkeen. 4 rengasta vanteilla tavaratilassa.",
			at(wed, 12, 4), facts(notBefore(at(wed, 14, 0))),
		},
		{
			"maksun jälkeen, ei ennen huomenna klo 13.30. auto pihalla ruudussa 8, avain tiskiltä.",
			at(thu, 16, 57), facts(notBefore(at(fri, 13, 30))),
		},
	})
}

func TestParsePickupHours(t *testing.T) {
	runCases(t, []parseCase{
		{"huom aukiolajat arkisin klo 8-15.30.", at(tue, 10, 50), facts(pickup(hm(8, 0), hm(15, 30)))},
		{"Liike auki arkisin 9.00-16.30.", at(thu, 11, 8), facts(pickup(hm(9, 0), hm(16, 30)))},
		{"open 9:00 - 15:00. car in bay 24, keys at the desk", at(tue, 17, 57), facts(pickup(hm(9, 0), hm(15, 0)))},
		{"Opening hours Mon-Fri 9:00–16:00, thanks.", at(tue, 14, 42), facts(pickup(hm(9, 0), hm(16, 0)))},
		{"nouto klo 9 - 16.30, kiitos taas", at(tue, 15, 2), facts(pickup(hm(9, 0), hm(16, 30)))},
		{"Open weekdasy 10:00-16:00", at(mon, 9, 50), facts(pickup(hm(10, 0), hm(16, 0)))},
		{"Arkipäivisin klo 10-14.30 nouto onnistuu / Öppet vardagar 10-14.30", at(mon, 9, 53), facts(pickup(hm(10, 0), hm(14, 30)))},
		{"hämtning 10-14", at(mon, 10, 31), facts(pickup(hm(10, 0), hm(14, 0)))},
		{"Nouto arkisin klo 9-16.30 / Open Mon-Fri 9am-4:30pm.", at(fri, 9, 11), facts(pickup(hm(9, 0), hm(16, 30)))},
		{"Auto valmiina, haettavissa heti. Arkisin 9:stä 14:ään nouto onnsituu.", at(fri, 10, 24), facts(pickup(hm(9, 0), hm(14, 0)))},
		{"Pickup weekdays 10:00 - 17:00 only", at(tue, 17, 54), facts(pickup(hm(10, 0), hm(17, 0)))},
		{"öppet vardagar kl 8-16", at(mon, 15, 54), facts(pickup(hm(8, 0), hm(16, 0)))},
		{"Klar, kan hämtas direkt. Hämtning mån-fre 7–14.30. Tack på förhand.", at(mon, 12, 0), facts(pickup(hm(7, 0), hm(14, 30)))},
		{"Nouto ennen klo 15.30", at(mon, 11, 27), facts(pickup(None, hm(15, 30)))},
		{"Haettava ennen klo 16.30", at(tue, 16, 35), facts(pickup(None, hm(16, 30)))},
		{"pitää hakea ennen kolmea", at(thu, 13, 9), facts(pickup(None, hm(15, 0)))},
		{"Colelct before 17:00, thank you.", at(wed, 14, 43), facts(pickup(None, hm(17, 0)))},
		{"Hi. We close at 15:30!", at(tue, 17, 44), facts(pickup(None, hm(15, 30)))},
		{"We close at 16:30, call 15 min before you arrive", at(mon, 16, 27), facts(pickup(None, hm(16, 30)))},
		{"Moikka. Liike kiinni klo 14.30, haettava ennen sitä. Hyvää päivänjatkoa", at(thu, 12, 26), facts(pickup(None, hm(14, 30)))},
		{"Täällä ei ketään neljän jälkeen, terveisin myynti", at(wed, 9, 52), facts(pickup(None, hm(16, 0)))},
		{"Ollaan täällä kello 15 saakka.", at(wed, 9, 9), facts(pickup(None, hm(15, 0)))},
		{"hejsan. öppet till klockan två. tack på förhand", at(wed, 10, 16), facts(pickup(None, hm(14, 0)))},
		{"Auki vain klo 14.30 asti!", at(fri, 11, 20), facts(pickup(None, hm(14, 30)))},
		{"19 jäälkeen ei enää luovutuksia. Kiitos", at(mon, 12, 0), facts(pickup(None, hm(19, 0)))},
		{"Shop closes 7pm. Br, sales", at(thu, 14, 12), facts(pickup(None, hm(19, 0)))},
		{"Hello. We close at 20:00 / Nouto ennen klo 20.00!", at(mon, 12, 4), facts(pickup(None, hm(20, 0)))},
	})
}

func TestParseDeliveryHours(t *testing.T) {
	runCases(t, []parseCase{
		{"receiving until 15:00 only, tahnks in advance", at(mon, 11, 12), facts(delivery(None, hm(15, 0)))},
		{"Vastaanotto onnistuu ma–pe 9-17. Auto pihalla ruudussa 27, avain tiskiltä.", at(mon, 15, 27), facts(delivery(hm(9, 0), hm(17, 0)))},
		{"Destination closes at 16:30", at(mon, 16, 34), facts(delivery(None, hm(16, 30)))},
		{"Moikka. Kohde sulkee klo 17. Noudettavissa arkisin 8 - 19.", at(mon, 16, 47), facts(delivery(None, hm(17, 0)), pickup(hm(8, 0), hm(19, 0)))},
		{"Receiving shop open 10 am-4:30 pm. Pickup 8 am-6 pm only", at(tue, 14, 40), facts(delivery(hm(10, 0), hm(16, 30)), pickup(hm(8, 0), hm(18, 0)))},
		{"Myyjä kotona huomenna 9.00-12.00. Perillä liike kiinni klo 16.00 jälkeen.", at(thu, 9, 15), facts(delivery(None, hm(16, 0)))},
		{"hei. kohde auki arkisin klo 8.00-18.00. auto valmis, nouto heti kun sopii.", at(fri, 17, 23), facts(delivery(hm(8, 0), hm(18, 0)))},
		{"Destination open weekdays 10:00 - 16:30. Collect before 20:00", at(mon, 11, 35), facts(delivery(hm(10, 0), hm(16, 30)), pickup(None, hm(20, 0)))},
		{"Vastaanotto klo 18 asti.", at(tue, 13, 40), facts(delivery(None, hm(18, 0)))},
		{"Receivng weekdays 10:00-15:00 only. Thanks", at(wed, 10, 45), facts(delivery(hm(10, 0), hm(15, 0)))},
		{"Toimitusosoite auki 9:stä 20:een.", at(mon, 10, 4), facts(delivery(hm(9, 0), hm(20, 0)))},
		{"Mottagaren har öppet vardagar 7-15", at(fri, 13, 17), facts(delivery(hm(7, 0), hm(15, 0)))},
		{"vastaanottaava liike auki arkisin 9-17.", at(tue, 12, 0), facts(delivery(hm(9, 0), hm(17, 0)))},
		{"Vastaanottaja paikalla vain 16.30 saakka. Hyvää päivänjatkoa.", at(tue, 16, 49), facts(delivery(None, hm(16, 30)))},
		{"Ingen hos mottagaren efter kl 16.00.", at(mon, 12, 0), facts(delivery(None, hm(16, 0)))},
		{"Kohteen aukioloajat arkisin klo 7 - 15. Rengassarja takapenkillä, kulkee mukana.", at(wed, 11, 28), facts(delivery(hm(7, 0), hm(15, 0)))},
		{"hämtas före kl 19. mottagnign vardagar kl 10-16.30. hälsningar", at(fri, 13, 44), facts(pickup(None, hm(19, 0)), delivery(hm(10, 0), hm(16, 30)))},
		{"vastaanotto arkisin 9-21. liike auki arkisin 8-16", at(thu, 9, 18), facts(delivery(hm(9, 0), hm(21, 0)), pickup(hm(8, 0), hm(16, 0)))},
		{
			"Vastaanottaja paikalla vain puoli viiteen saakka / Mottagning till kl 16.30. Liike kiinni kello 20, haettava ennen sitä",
			at(fri, 15, 57), facts(delivery(None, hm(16, 30)), pickup(None, hm(20, 0))),
		},
	})
}

func TestParseDeadline(t *testing.T) {
	runCases(t, []parseCase{
		{"perillä torstaina klo 15.30 mennessä", at(mon, 14, 11), facts(deadline(at(thu, 15, 30)))},
		{"Hi, ready now, must be at destination by tomorrow 19:00, call 20 min before you arrive", at(mon, 16, 31), facts(deadline(at(tue, 19, 0)))},
		{"Asiakas tulee hakemaan, perillä viim. keskiviikkona klo 12:30. Kiitos etukäteen.", at(mon, 17, 22), facts(deadline(at(wed, 12, 30)))},
		{"At destination 18:00 latest. Collect before 19:00", at(wed, 14, 56), facts(deadline(at(wed, 18, 0)), pickup(None, hm(19, 0)))},
		{"Toimitus perille viim. huomenna klo 17!", at(thu, 10, 6), facts(deadline(at(fri, 17, 0)))},
		{"Oltava perillä viimeistään klo 16.30.", at(thu, 11, 33), facts(deadline(at(thu, 16, 30)))},
		{"Mottagaren har öppet kl 7-21. Frmame senast kl 17", at(fri, 12, 57), facts(deadline(at(fri, 17, 0)), delivery(hm(7, 0), hm(21, 0)))},
		{"Must be at destination by tommorrow 13:00.", at(mon, 15, 13), facts(deadline(at(tue, 13, 0)))},
		{"must be at destination by tomorrow 16.00. key box code 3452", at(mon, 18, 2), facts(deadline(at(tue, 16, 0)))},
		{"framme senast på torsdag kl 13, ring kunden en timme innan.", at(tue, 16, 44), facts(deadline(at(thu, 13, 0)))},
		{"måste vara framme senast i övermorgon 15. hämtning senast 20.", at(wed, 13, 14), facts(deadline(at(fri, 15, 0)), pickup(None, hm(20, 0)))},
		{"Toimitus viimeistään huomenna klo 18:30", at(tue, 10, 39), facts(deadline(at(wed, 18, 30)))},
		{"Perillä huomenna klo 15 mennssä. Tilausnro 149171.", at(wed, 14, 8), facts(deadline(at(thu, 15, 0)))},
		{"At detination tomorrow 12:30 latest.", at(thu, 17, 37), facts(deadline(at(fri, 12, 30)))},
		{"Asiakas noutaa huomenna 20.00, auton oltava silloin perillä", at(tue, 10, 27), facts(deadline(at(wed, 20, 0)))},
		{"Deliver no later than Monday 8 pm. Collect before 7 pm", at(fri, 10, 8), facts(deadline(at(nextMon, 20, 0)), pickup(None, hm(19, 0)))},
		{"Pitää olla perillä ylihuomenna klo 20.00 mennessä, avaimet huollosta", at(tue, 9, 4), facts(deadline(at(thu, 20, 0)))},
		{
			"Latest arrival the day after tomorrow 8pm. Nobody here after 4:30pm. Thank you",
			at(wed, 16, 14), facts(deadline(at(fri, 20, 0)), pickup(None, hm(16, 30))),
		},
		{
			"must be at destination by fri 20:00. we close at 14:30. thanks.",
			at(wed, 19, 49), facts(deadline(at(fri, 20, 0)), pickup(None, hm(14, 30))),
		},
		{
			"Perillä keskiviikkona klo 18.30 mennessä. Huolto kesken, valmis huomenna klo 7.00. Renkaat takakontissa, menee mukana",
			at(mon, 13, 23), facts(deadline(at(wed, 18, 30)), notBefore(at(tue, 7, 0))),
		},
	})
}
