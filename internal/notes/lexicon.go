package notes

import (
	"sort"
	"strings"
)

// feat is a bit set of what a word contributes to the meaning of a note.
type feat uint64

const (
	fDest       feat = 1 << iota // the delivery site
	fArrive                      // being at the destination
	fLatest                      // "at the latest", "by"
	fOpen                        // open, opening hours
	fOpens                       // the act of opening
	fClose                       // closes, closed
	fPickup                      // collecting the car
	fReady                       // the car is (not yet) ready
	fEarliest                    // "at the earliest", "from"
	fBefore                      // before
	fUntil                       // until
	fAfter                       // after
	fPay                         // payment, handover
	fNobody                      // nobody is there
	fSeller                      // a private seller's own slot
	fCustomer                    // the end customer
	fCall                        // phone instructions
	fNeg                         // negation
	fCorr                        // the note corrects itself
	fClock                       // "klo", "kl", "at"
	fHalf                        // "puoli", "halv", "half"
	fMeridiem                    // am (val 0) or pm (val 1)
	fDay                         // relative day, val = days after arrival
	fWeekday                     // weekday, val = 0 (Monday) .. 6
	fShort                       // short weekday form that is also an ordinary word
	fWeekdays                    // "arkisin", "weekdays"
	fHour                        // hour written as a word, val = its number
	fStrong                      // hour word in a case that only a clock time takes
	fNoTimePrev                  // a number after this word is not a time
	fNoTimeNext                  // a number before this word is not a time
	fRangeSep                    // "to", "till" between two times
	fManual                      // manual gearbox
	fAuto                        // automatic gearbox
	fPlate                       // number plates
	fPlateNeed                   // transfer or trade plates
	fUnreg                       // unregistered
	fMissing                     // missing, removed, without
	fSummer                      // summer tyres
	fWinter                      // winter tyres
	fCargo                       // something travels in the car
	fTyre                        // tyres, wheels
)

// fTopic are the features that say what a clause is about. A clause without
// one borrows the topic of the clause before it in the same sentence.
const fTopic = fDest | fArrive | fOpen | fOpens | fClose | fPickup | fReady | fPay | fNobody | fSeller | fCustomer

type entry struct {
	word string
	f    feat
	val  int
}

var (
	lexExact  map[string]entry
	lexByHead [256][]entry // entries by first byte, for fuzzy lookup
	lexStems  []entry
)

func add(f feat, val int, words string) {
	for _, w := range strings.Fields(words) {
		e := lexExact[w]
		e.word = w
		e.f |= f
		if f&(fDay|fWeekday|fHour|fMeridiem) != 0 {
			e.val = val
		}
		lexExact[w] = e
	}
}

func stem(f feat, val int, stems string) {
	for _, s := range strings.Fields(stems) {
		lexStems = append(lexStems, entry{word: s, f: f, val: val})
	}
}

func init() {
	lexExact = make(map[string]entry, 1024)
	addSites()
	addTimes()
	addFlags()
	addNeutral()
	for _, e := range lexExact {
		lexByHead[e.word[0]] = append(lexByHead[e.word[0]], e)
	}
	for i := range lexByHead {
		sort.Slice(lexByHead[i], func(a, b int) bool { return lexByHead[i][a].word < lexByHead[i][b].word })
	}
}

func addSites() {
	add(fDest, 0, "kohde kohteen kohteessa kohteeseen toimitusosoite toimitusosoitteessa vastaanotto vastaanoton vastaanottava vastaanottaja vastaanottajalla")
	add(fDest, 0, "receiving receiver recipient dropoff drop mottagning mottagningen mottagaren mottagare leveransadressen")
	add(fDest|fArrive, 0, "perilla destination destinationen maaranpaa maaranpaassa")
	add(fArrive, 0, "perille framme arrive arrives arrival toimitus toimitettava toimitettu deliver delivered delivery leverans levereras ankomst")
	add(fLatest, 0, "viim viimeistaan mennessa latest senast by later deadline takaraja")
	add(fOpen, 0, "auki avoinna open oppet aukioloajat aukioloaika aukiolo oppettider oppettiderna opening")
	add(fOpens, 0, "aukeaa avautuu avataan avaamme aukeamme opens oppnar oppnas")
	add(fClose, 0, "sulkee sulkeutuu suljetaan suljemme suljettu kiinni close closes closed closing stanger stangt stangd stangs shut shuts")
	add(fPickup, 0, "nouto noudon noutoa noudettavissa noudettava noutaa haettava haettavissa hakea hakee pickup pick picking picks collect collects collected collection hamtning hamtningen hamtas hamta hamtar")
	add(fReady, 0, "valmis valmiina valmistuu ready klar klart fardig workshop verkstad verkstaden")
	add(fEarliest, 0, "aikaisintaan earliest tidigast alkaen lahtien from fran framat vasta forst onwards")
	add(fBefore, 0, "ennen before fore innan")
	add(fUntil, 0, "asti saakka until till til")
	add(fAfter, 0, "jalkeen after efter")
	add(fPay, 0, "maksu maksun maksaa maksettu maksanut payment pays paid pay betalning betalningen betalar betalat betald luovutus luovutetaan luovutuksia lamnas utlamning released release handover clears")
	add(fNobody, 0, "ketaan nobody noone ingen enaa")
	add(fSeller, 0, "myyja seller owner saljaren agaren omistaja kotona home hemma")
	add(fCustomer, 0, "asiakas customer kunden kund")
	add(fCall, 0, "soita soittakaa call ring ringa phone")
	add(fNeg, 0, "ei not inte ej eika")
	add(fNeg|fMissing, 0, "no")
	add(fCorr, 0, "eikun eiku korjaus korjaan correction corrected mean meant sorry tarkoitan tarkoitin siis rattelse nej forlat menar menade oops typo anteeksi pardon rather actually wait")
	stem(fDest, 0, "vastaanot mottag kohtee toimitusoso receiv")
	stem(fDest|fArrive, 0, "destinat")
	stem(fOpen, 0, "aukiolo oppetti")
	stem(fClose, 0, "sulje sulkeu")
	stem(fPickup, 0, "noudettav haettav")
	stem(fReady, 0, "valmist")
	stem(fPay, 0, "maksu makset betaln luovut")
	stem(fLatest, 0, "viimeist")
	stem(fEarliest, 0, "aikaisint")
}

func addTimes() {
	add(fClock, 0, "klo kello kl klockan at vaan utan but")
	add(fHalf, 0, "puoli halv half")
	add(fMeridiem, 0, "am")
	add(fMeridiem, 1, "pm")
	add(fDay, 0, "tanaan idag dag today")
	add(fDay, 1, "huomenna tomorrow tmrw tmr tmw imorgon morgon imorn")
	add(fDay, 2, "ylihuomenna overmorgon iovermorgon")
	stem(fDay, 2, "ylihuom")
	stem(fDay, 1, "huomis")
	add(fWeekdays, 0, "arkisin arkipaivisin vardagar weekdays daily")
	add(fRangeSep, 0, "to till til")
	days := [daysPerWeek][3]string{
		{"maanantai maanantaina maanantaihin maanantaiksi mandag mandagen monday", "ma man mon", "maanant"},
		{"tiistai tiistaina tiistaihin tiistaiksi tisdag tisdagen tuesday", "ti tis tue tues", "tiista"},
		{"keskiviikko keskiviikkona keskiviikkoon keskiviikoksi onsdag onsdagen wednesday", "ke ons wed weds", "keskiviik"},
		{"torstai torstaina torstaihin torstaiksi torsdag torsdagen thursday", "to tors tor thu thur thurs", "torsta"},
		{"perjantai perjantaina perjantaihin perjantaiksi fredag fredagen friday", "pe fre fri", "perjant"},
		{"lauantai lauantaina lauantaihin lordag lordagen saturday", "la lor sat", "lauant"},
		{"sunnuntai sunnuntaina sunnuntaihin sondag sondagen sunday", "su son sun", "sunnunt"},
	}
	for wd, d := range days {
		add(fWeekday, wd, d[0])
		add(fWeekday|fShort, wd, d[1])
		stem(fWeekday, wd, d[2])
	}
	// Finnish hour words: nominative and genitive need a time word next to
	// them; the local cases ("kolmelta", "kahdesta", "neljaan") are clock times.
	fi := [12][2]string{
		{"yksi yhden yhta", "yhdelta yhdesta yhteen"},
		{"kaksi kahden kahta", "kahdelta kahdesta kahteen"},
		{"kolme kolmen kolmea", "kolmelta kolmesta kolmeen"},
		{"nelja neljan neljaa", "neljalta neljasta neljaan"},
		{"viisi viiden viitta", "viidelta viidesta viiteen"},
		{"kuusi kuuden kuutta", "kuudelta kuudesta kuuteen"},
		{"seitseman seitsemaa", "seitsemalta seitsemasta seitsemaan"},
		{"kahdeksan kahdeksaa", "kahdeksalta kahdeksasta kahdeksaan"},
		{"yhdeksan yhdeksaa", "yhdeksalta yhdeksasta yhdeksaan"},
		{"kymmenen kymmenta", "kymmenelta kymmenesta kymmeneen"},
		{"yksitoista yhdentoista yhtatoista", "yhdeltatoista yhdestatoista yhteentoista"},
		{"kaksitoista kahdentoista kahtatoista", "kahdeltatoista kahdestatoista kahteentoista"},
	}
	for i, forms := range fi {
		add(fHour, i+1, forms[0])
		add(fHour|fStrong, i+1, forms[1])
	}
	other := [12]string{
		"one ett", "two tva", "three tre", "four fyra", "five fem", "six sex",
		"seven sju", "eight atta", "nine nio", "ten tio", "eleven elva", "twelve tolv",
	}
	for i, forms := range other {
		add(fHour, i+1, forms)
	}
	add(fHour, noonHour, "noon midday keskipaivalla keskipaivaan")
	add(fNoTimePrev, 0, "ruudussa ruutu bay plats koodi code kod viite viitenro viitenumero ref referens reference varaus varausnro tilaus tilausnro tilausnumero order ordernr booking bokning nro nr numero number puh tel box slot lot spot")
	add(fNoTimeNext, 0, "min mins minutes minuuttia minuter tkm km sarjaa kpl st tuntia timmar hours sets set")
	add(fNoTimeNext|fTyre, 0, "rengasta renkaita tyres tires wheels dack")
	add(fNoTimeNext|fTyre|fCargo, 0, "kesarengasta talvirengasta")
}

func addFlags() {
	add(fManual, 0, "manuaali manuaalia manuaalin manuaalivaihteinen manuaalivaihteet manuaalivaihteisto manuaalikortin manuaalikortti manual manuell manuellt manuella kasivaihteinen kasivaihteet stick")
	stem(fManual, 0, "manuaal manuell")
	add(fAuto, 0, "automaatti automaattia automaattivaihteinen automatic automat automatisk automatvaxlad")
	add(fPlate, 0, "kilvet kilpia kilpi rekisterikilvet rekisterikilpia rekkareita rekkarit plates plate skyltar skyltarna registreringsskyltar nummerskyltar")
	add(fPlateNeed, 0, "siirtokilvet siirtokilpia siirtokilpi koekilvet koekilpia koenumerokilvet siirtolupa flyttskyltar provskyltar saluvagnsskyltar")
	stem(fPlateNeed, 0, "siirtokil koekil siirtolu")
	stem(fPlate, 0, "rekisterikil")
	add(fUnreg, 0, "unregistered deregistered unplated rekisteroimaton rekisterista avregistrerad avstalld")
	add(fMissing, 0, "puuttuu puuttuvat ilman irrotettu irti poistettu removed without missing inga utan saknar saknas borttagna")
	add(fSummer, 0, "kesarenkaat kesarenkailla kesarenkaissa kesakumit kesakumeilla sommardack sommardacken summer summers")
	add(fWinter, 0, "talvirenkaita talvirenkaat winters winter vinterdack vinterdacken")
	add(fCargo, 0, "takakontissa tavaratilassa kontissa takapenkilla kyydissa mukaan mukana vanteineen vanteilla lisaksi trunk boot back bakluckan bagaget in i med foljer medfoljer ingar included inside along loaded sisalla")
}

// addNeutral lists common harmless words. They carry no meaning, but a typed
// word is matched to its nearest known word, so knowing the chatter keeps
// "rekisteriote" (a document) away from "rekisterista" (unregistered).
func addNeutral() {
	add(0, 0, "rekisteriote huoltokirja kauppakirja paperit hanskalokerossa avaimet avain autossa auto auton huollon huollosta huolto kesken")
	add(0, 0, "toimistosta tiskilta myyjalta myynti liike liikkeen liikkeeseen liikkeelle pihalla pihasta kiitos kiitti paljon etukateen terveisin")
	add(0, 0, "vastaanottajalle asiakkaalle asiakkaan saapumista lahella noutaessa hakemaan takaisin vaihtari vaihtoauto meille siirto normaali tavallinen")
	add(0, 0, "rengassarja lisarenkaat ylimaarainen renkaat sarja toinen kulkee menee extra dacksats dacken ligger bilen second spare glovebox papers keys reception")
	add(0, 0, "hyvaa paivanjatkoa moikka terve hello there thanks thank cheers halsningar hejsan transfer standard nothing special transport vanlig")
	add(0, 0, "heti sopii onnistuu oltava pitaa must have shop only vain huom still viela fortfarande alla receives weekday service kuljettajalla kuski tarvitaan tarvii")
}

// lookup classifies a folded word: exactly, by a known stem, or by the nearest
// known word within the typo budget for its length.
func lookup(w string) (feat, int) {
	if e, ok := lexExact[w]; ok {
		return e.f, e.val
	}
	if len(w) == 0 || len(w) > maxFuzzyLen {
		return 0, 0
	}
	for _, s := range lexStems {
		if strings.HasPrefix(w, s.word) {
			return s.f, s.val
		}
	}
	budget := fuzzyBudget(len(w))
	if budget == 0 {
		return 0, 0
	}
	best, bestDist := entry{}, budget+1
	for _, e := range lexByHead[w[0]] {
		if len(e.word) < minTypoLen {
			continue
		}
		if d := osaDistance(w, e.word, bestDist-1); d < bestDist {
			best, bestDist = e, d
		}
	}
	return best.f, best.val
}
