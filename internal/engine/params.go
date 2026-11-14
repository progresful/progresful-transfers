// Package engine is the dispatcher: it keeps the books of one world and decides
// quotes, gigs and fees. The same engine plays practice worlds and the final.
package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Params are the tunable numbers of the policy. Sweeps vary them by name (JSON tag).
type Params struct {
	// --- Quoting ---
	// PriceMult is the quoted price as a multiple of the option's list price (level 4+).
	PriceMult float64 `json:"priceMult"`
	// CostMult is the cost of an order as a multiple of the request's typical fee: the
	// driver's fee plus what the scarce driver time is worth. The quote is priced on it.
	CostMult float64 `json:"costMult"`
	// Volume control (shortage levels): the worth of driver time differs from world to world,
	// so CostMult follows the day's wins. The drivers of a world carry about TargetPerDriver
	// orders each in the week; when the day's wins run behind that pace the cost falls
	// (quotes get cheaper), ahead of it the cost rises, by CostGain, within CostMin..CostMax.
	// TargetPerDriver 0 keeps CostMult fixed.
	TargetPerDriver float64 `json:"targetPerDriver"`
	CostGain        float64 `json:"costGain"`
	CostMin         float64 `json:"costMin"`
	CostMax         float64 `json:"costMax"`
	// Fee model (shortage levels, feemodel.go): each gig's own expected booking fee
	// instead of one multiple for all. CostByGig 1 prices a request on that fee plus
	// CostSpread typical fees (the worth of scarce driver time) instead of CostMult.
	// OpenByGig 1 opens a gig at OpenShare of its expected fee and holds at MidShare of
	// it, instead of FeeStart and FeeMid.
	CostByGig  int     `json:"costByGig"`
	CostSpread float64 `json:"costSpread"`
	OpenByGig  int     `json:"openByGig"`
	OpenShare  float64 `json:"openShare"`
	MidShare   float64 `json:"midShare"`
	// FloorMult is the least price as a multiple of the typical fee (level 4+): the fee
	// itself. A quote goes under CostMult only when the order is worth more than its
	// margin, for the trust it builds.
	FloorMult float64 `json:"floorMult"`
	// SlowOptions is how many of a request's delivery options are quoted, counted from the
	// slowest: 0 quotes every option that passes the checks, 1 only the slowest, 2 the two
	// slowest. At equal price ratios a dealer always takes the fastest option quoted, and
	// the fastest is the hardest to deliver on time.
	SlowOptions int `json:"slowOptions"`
	// SameDayPremium multiplies the same-day price: the riskiest option, and at equal
	// prices the one dealers always take.
	SameDayPremium float64 `json:"sameDayPremium"`
	// MinMarginCents and MinMarginShare are the least expected margin to take a request:
	// price - cost must reach both the absolute amount and the share of the cost.
	MinMarginCents int     `json:"minMarginCents"`
	MinMarginShare float64 `json:"minMarginShare"`
	// MinSlackMin declines an option when fewer minutes than this lie between the
	// earliest pickup and the last pickup that is still on time.
	MinSlackMin int `json:"minSlackMin"`
	// MinLeadMin and MinLeadDrive decline an option when the minutes from the answer to
	// the last on-time pickup are fewer than MinLeadMin + MinLeadDrive x the drive: a long
	// drive with a few hours left finds its driver late, at a high fee. 0 and 0: off.
	MinLeadMin   int     `json:"minLeadMin"`
	MinLeadDrive float64 `json:"minLeadDrive"`
	// MinSupply declines a request when fewer drivers than this live within the app's
	// notification radius of the pickup or of the delivery (whichever has more). For a
	// premium car only experienced drivers count, against MinExperienced.
	MinSupply      int `json:"minSupply"`
	MinExperienced int `json:"minExperienced"`
	// TradeIns 1 quotes trade-ins (two cars, one visit at the customer's booked hour); 0
	// declines them. In the shortage almost half arrived late: a driver needs a long free
	// block around a fixed hour, and the fee ran to 1.4 times the typical one.
	TradeIns int `json:"tradeIns"`
	// SplitTradeIns 1 posts the two cars of a trade-in as two gigs even where one gig may
	// carry both (level 3+): a combined trade-in needs a driver with a long free block
	// around the customer's hour and proved hard to book.
	SplitTradeIns int `json:"splitTradeIns"`
	// RiskSlope adds risk x RiskSlope typical fees to the cost of an order: a steeper price
	// for risky orders than RiskCost (which is in list prices), so a lower CostMult adds
	// volume only where it is safe.
	RiskSlope float64 `json:"riskSlope"`
	// JoinCost is the cost multiple in a world that the system joined after its first day
	// with no order of its own (a test final starts on Thursday): the dealers have had
	// days of unanswered requests and accept only low prices, and the drivers are free.
	// 0 keeps CostMult. A full week, and a restart in the middle of one, never use it.
	JoinCost float64 `json:"joinCost"`
	// FirstDayCost is the cost of an order on the Monday of a full week (0: CostMult as on
	// every day). No dealer trusts the team yet, so prices are at their lowest: orders won
	// then are the trust that the rest of the week is sold on.
	FirstDayCost float64 `json:"firstDayCost"`
	// RosterCost is the cost of an order in a world with at least RosterFrom drivers (0:
	// CostMult in every world). The driver list is known from the first minute; a cost of
	// 1.1 instead of 1.2 earned more in the worlds with 720 drivers or more and lost in
	// the worlds with fewer, where every extra order makes the others late.
	RosterCost float64 `json:"rosterCost"`
	RosterFrom int     `json:"rosterFrom"`
	// ReplayQuotes 1 restores a fault of the engines up to e16: when joining a world with
	// a past, they priced the past's requests anew and took the dealers' answers of that
	// time as answers to those prices, which taught the price model losses that never
	// happened. Kept only to reproduce those runs.
	ReplayQuotes int `json:"replayQuotes"`
	// Premium 1 quotes premium cars (experienced drivers only); 0 declines them. In the
	// shortage they arrived late almost one time in five and earned a third less.
	Premium int `json:"premium"`
	// RiskModel picks the lateness model: 2 the supply model (the champion's), 3 the one
	// that also knows weekday, hour and delivery option (its risks are on a lower scale:
	// use RiskMax near 0.2 and no Thursday/Friday factors with it).
	RiskModel int `json:"riskModel"`
	// CostToday and CostLater multiply the cost of an order by when its first car is ready:
	// on the day of the request, or on a later day. A car ready later finds a driver fast
	// and cheap (half within 30 minutes, at 0.85 of the typical fee) and is rarely late; a
	// car ready today is slow, dearer (1.05) and late three times as often.
	CostToday float64 `json:"costToday"`
	CostLater float64 `json:"costLater"`
	// CostLong and CostShort multiply the cost of an order on a long lane (300 km or more)
	// and on a short one (under 60 km): long hauls carry most of the margin, short hops little.
	CostLong  float64 `json:"costLong"`
	CostShort float64 `json:"costShort"`
	// FridayCost multiplies the cost of an order quoted on a Friday: drivers for the
	// weekend take three times as long to find and cost about a quarter more.
	FridayCost float64 `json:"fridayCost"`
	// RiskMax drops a delivery option whose modelled chance of arriving late is above it.
	// RiskCost prices the rest: the expected cost of a quote grows by risk x RiskCost x list price.
	RiskMax  float64 `json:"riskMax"`
	RiskCost float64 `json:"riskCost"`
	// RiskThursday and RiskFriday scale the risk cap on the last two days of requests.
	RiskThursday float64 `json:"riskThursday"`
	RiskFriday   float64 `json:"riskFriday"`
	// The delivery minimum is 90% on time, or the week is not ranked. The guard watches the
	// expected on-time share of the week (late orders, and open orders by their chance to
	// end late) and tightens RiskMax as it nears the minimum: full RiskMax at GuardFull,
	// GuardMin of it at GuardFloor. GuardFull 0 turns it off.
	GuardFull  float64 `json:"guardFull"`
	GuardFloor float64 `json:"guardFloor"`
	GuardMin   float64 `json:"guardMin"`
	// The other side of the guard: a week whose outlook stays far above the minimum has
	// drivers to spare, and is held back by price, not by capacity. Between LoosenFrom and
	// LoosenFull on-time outlook, the risk cap grows up to LoosenRisk times and the cost
	// of an order falls by up to LoosenCost (in typical fees). LoosenRisk 1 and LoosenCost 0
	// turn it off.
	LoosenFrom float64 `json:"loosenFrom"`
	LoosenFull float64 `json:"loosenFull"`
	LoosenRisk float64 `json:"loosenRisk"`
	LoosenCost float64 `json:"loosenCost"`
	// DeclineMult is the losing quote used to decline at level 4+ (multiple of list price).
	DeclineMult float64 `json:"declineMult"`
	// Market model (level 4+): a quote at ratio r of the list price wins with a chance that is
	// logistic in log r. MarketStart is the first guess of the ratio that wins half the time
	// (0: by level), MarketScale the width, MarketRate / DealerRate how fast the world level
	// and each dealer's offset follow the outcomes.
	MarketStart float64 `json:"marketStart"`
	MarketScale float64 `json:"marketScale"`
	MarketRate  float64 `json:"marketRate"`
	DealerRate  float64 `json:"dealerRate"`
	// MarketModel 2 replaces the running rates by Gaussian beliefs about each part of the
	// price level (beliefs.go): the world, the day, the dealer and each end of the lane.
	// BeliefScale is the width of a dealer's own choice; BeliefWorld, BeliefDay,
	// BeliefDealer and BeliefPlace are the prior deviations of the parts (log ratio);
	// BeliefDrift is the deviation a dealer's and a place's part gain each morning.
	MarketModel  int     `json:"marketModel"`
	BeliefScale  float64 `json:"beliefScale"`
	BeliefWorld  float64 `json:"beliefWorld"`
	BeliefDay    float64 `json:"beliefDay"`
	BeliefDealer float64 `json:"beliefDealer"`
	BeliefPlace  float64 `json:"beliefPlace"`
	BeliefDrift  float64 `json:"beliefDrift"`
	// DayRate is how fast the market level follows the current day alone (per dealer and
	// region too): the competitor's day factor, the price war and the outage. 0 turns it off.
	DayRate float64 `json:"dayRate"`
	// DestRate is how fast the level learns a shift of its own for each delivery place
	// from the week's wins and losses (the part of a lane's price that is new in each
	// world). 0: off.
	DestRate float64 `json:"destRate"`
	// LaneWeight is how much of the fitted lane price effects (lanes.go) the quote uses:
	// 1 prices each lane against its own level, 0 one level for all lanes.
	LaneWeight float64 `json:"laneWeight"`
	// Adaptive 0 quotes the fixed PriceMult; 1 quotes the ratio with the best expected gain.
	Adaptive int `json:"adaptive"`
	// TrustBoost scales how much of a dealer's trust (level 8: +1% per on-time order) the
	// quote claims; TrustInvest scales the extra worth of an order that builds trust.
	TrustBoost  float64 `json:"trustBoost"`
	TrustInvest float64 `json:"trustInvest"`
	// TrustInvestRisk limits TrustInvest to requests whose modelled lateness risk is at or
	// under it (0: every request).
	TrustInvestRisk float64 `json:"trustInvestRisk"`
	// TrustRush (rush.go): a car of a dealer that sends at least this share of all requests
	// is posted to arrive on the day it is ready, while the dealer's trust can still grow
	// (0: off). TrustRushMin is the least room in minutes between the first possible
	// pickup and the last one that still arrives that day.
	TrustRush    float64 `json:"trustRush"`
	TrustRushMin int     `json:"trustRushMin"`
	// ShareCap and ShareMarkup: the competitor cuts its prices for a dealer whose requests
	// we mostly win. Above ShareCap of a dealer's requests won, quotes to it go up by
	// ShareMarkup, to stay under the competitor's radar. ShareCap 1 turns it off.
	ShareCap    float64 `json:"shareCap"`
	ShareMarkup float64 `json:"shareMarkup"`
	// TrustRiskCut tightens the risk cap for a trusted dealer (a late order halves its
	// count): at full trust the cap is (1 - TrustRiskCut) of the usual. TrustFeeBoost
	// raises the top fee of a trusted dealer's gig: FeeMax times it at full trust.
	TrustRiskCut  float64 `json:"trustRiskCut"`
	TrustFeeBoost float64 `json:"trustFeeBoost"`
	// TrustGuard scales the worth of the trust a late order would destroy (it halves the
	// dealer's count), added to the cost of risk when pricing.
	TrustGuard float64 `json:"trustGuard"`
	// TrustWinShare is the share of a dealer's later requests the engine expects to win.
	TrustWinShare float64 `json:"trustWinShare"`
	// MinWinChance declines a request when even the best-paying price wins less often than this.
	MinWinChance float64 `json:"minWinChance"`

	// --- Fees (multiples of the gig's base fee, the typical fee of its cars) ---
	// Discovery clock: the fee opens at FeeStart and rises by FeeStep every StepMin up to FeeMid.
	FeeStart float64 `json:"feeStart"`
	FeeStep  float64 `json:"feeStep"`
	StepMin  int     `json:"stepMin"`
	FeeMid   float64 `json:"feeMid"`
	// Deadline pressure: from LeadMin before the last on-time pickup the fee rises from
	// FeeMid to FeeMax, reached FinalMin before it, in UrgentSteps steps.
	FeeMax      float64 `json:"feeMax"`
	LeadMin     int     `json:"leadMin"`
	FinalMin    int     `json:"finalMin"`
	UrgentSteps int     `json:"urgentSteps"`
	// HoldMax above FeeMid lets the fee keep rising between the discovery clock and the
	// deadline pressure: from FeeMid when the clock stops to HoldMax when the pressure
	// starts, in HoldSteps steps. A gig that found no driver at FeeMid needs more than
	// FeeMid, and most drivers book the day before they drive. 0 keeps the fee at FeeMid.
	HoldMax   float64 `json:"holdMax"`
	HoldSteps int     `json:"holdSteps"`
	// LeadDrive starts the deadline pressure earlier for a long gig: LeadMin plus this
	// multiple of the gig's road minutes before the last on-time pickup. A driver for a
	// long lane has a long way to the car too.
	LeadDrive float64 `json:"leadDrive"`
	// Late cars (no on-time pickup left): fee from LateStart to LateMax over LateRampMin.
	LateStart   float64 `json:"lateStart"`
	LateMax     float64 `json:"lateMax"`
	LateRampMin int     `json:"lateRampMin"`
	// A car not delivered when the world ends costs twice its price on top of the lost
	// revenue. A late gig that still finds no driver therefore keeps rising after LateMax,
	// to LastMult times the base fee (but not above LastPriceShare of the order's price)
	// LastLeadMin before the world ends.
	LastMult       float64 `json:"lastMult"`
	LastPriceShare float64 `json:"lastPriceShare"`
	LastLeadMin    int     `json:"lastLeadMin"`
	// No fee above what the delivery is worth: an on-time delivery earns the price and
	// avoids the penalty of twice the price, so its fee stops at OnTimePriceCap times the
	// order's price; a late one at LatePriceCap times it. This matters after a dealer
	// sends a car much farther than quoted.
	OnTimePriceCap float64 `json:"onTimePriceCap"`
	LatePriceCap   float64 `json:"latePriceCap"`
	// Real drives run over the map minutes. The delivery stop gets its own window that ends
	// BufferShare of the drive (at least BufferMin) before the deadline, so drivers plan to arrive early.
	BufferShare float64 `json:"bufferShare"`
	BufferMin   int     `json:"bufferMin"`
	// Snow (snow.go): a car whose way may lie through forecast snow gets SnowBuffer of its
	// drive as the buffer instead of BufferShare (0: no difference), and its modelled
	// lateness risk is multiplied by SnowRisk when its request is priced (0 or 1: no change).
	SnowBuffer float64 `json:"snowBuffer"`
	SnowRisk   float64 `json:"snowRisk"`
	// ThinRoster and ThinRisk (risk.go): in a world with fewer than ThinRoster drivers the
	// modelled lateness risk of a request is multiplied by ThinRisk (0: off).
	ThinRoster int     `json:"thinRoster"`
	ThinRisk   float64 `json:"thinRisk"`
	// Outage (outage.go) is the price of the dealers' backup over the competitor's price
	// that the engine assumes while the competitor is down in a region (the handbook says
	// about 1.3); 0 or 1 turns the watch off. OutagePrior is the odds of an outage in one
	// region on one day before any answer.
	Outage      float64 `json:"outage"`
	OutagePrior float64 `json:"outagePrior"`
	// Chains (level 3+, chains.go): Chains 1 puts two single-car orders in one gig when the
	// second pickup is within ChainHopMin road minutes of the first delivery and both cars
	// stay on time with at least ChainSlackMin minutes of room to start. The chain's fee is
	// ChainFeeShare of the two typical fees. If no driver takes it within ChainTryMin
	// minutes, the cars are posted again one by one.
	Chains int `json:"chains"`
	// ChainTries is how many chain attempts one order may be part of before it goes alone.
	// ChainMaxCars is the most cars in one chain: an open chain with room takes one more.
	ChainTries    int     `json:"chainTries"`
	ChainMaxCars  int     `json:"chainMaxCars"`
	ChainHopMin   int     `json:"chainHopMin"`
	ChainSlackMin int     `json:"chainSlackMin"`
	ChainTryMin   int     `json:"chainTryMin"`
	ChainFeeShare float64 `json:"chainFeeShare"`
	// ChainMinChance is the least chance of being booked (chains.go, chainChance) at which
	// a chain is tried; with it the likeliest pairing wins instead of the shortest hop. 0: off.
	ChainMinChance float64 `json:"chainMinChance"`
	// SplitClock is how much of a failed chain attempt's time the cars keep on their fee
	// clock when they go alone: 0 opens them at FeeStart again, 1 opens them at the fee a
	// car posted alone at the start of the attempt would show now.
	SplitClock float64 `json:"splitClock"`
	// ChainCostGain lowers the cost a request is priced on when it would make a chain with
	// an open gig: by ChainCostGain times the chance that the chain is booked. 0: off.
	ChainCostGain float64 `json:"chainCostGain"`
	// ChainOffers 1 sends a direct offer of an open gig to a driver who already holds one
	// of our gigs that ends within ChainHopMin road minutes of its pickup, at a time that
	// fits: for that driver the second car costs only the extra drive. The offer pays at
	// least ChainOfferMult times the gig's base fee.
	ChainOffers    int     `json:"chainOffers"`
	ChainOfferMult float64 `json:"chainOfferMult"`
	// Direct offers (level 3+). Offers 1 turns them on. A gig gets offers once it has been
	// open OfferAfterMin minutes, or at once when fewer than OfferReach drivers live within
	// the app's 30 km notification radius of its pickup. An offer pays at least OfferMult
	// times the base fee. OfferMaxScore bounds the travel minutes of a driver worth asking.
	Offers int `json:"offers"`
	// OfferViewers 1 sends offers only to drivers who opened the gig in the last
	// OfferViewMin minutes: they use the app now, the gig interests them, and in the app
	// they may hold out for a raise, which an offer does not let them do.
	OfferViewers  int     `json:"offerViewers"`
	OfferViewMin  int     `json:"offerViewMin"`
	OfferAfterMin int     `json:"offerAfterMin"`
	OfferReach    int     `json:"offerReach"`
	OfferMult     float64 `json:"offerMult"`
	OfferMaxScore int     `json:"offerMaxScore"`
	// Notes 1 reads the dealers' notes: ready and handover times, deadlines, plates and
	// manual gearboxes shape the gig; notes that shorten a site's hours or name summer
	// tyres are declined. 0 ignores notes.
	Notes int `json:"notes"`
	// NoteHours 1 also takes cars whose note shortens a dealer site's hours, posting them
	// with windows of their own for one day at a time; 0 declines them.
	NoteHours int `json:"noteHours"`
	// RetryAfterMin is how long to wait before a new pickup try after a failed visit.
	RetryAfterMin int `json:"retryAfterMin"`
	// Research 1 plays a study week: it quotes but posts no gig, so the reveal shows what
	// was true of every car won (the labels for the note reader) without moving anything.
	Research int `json:"research"`
}

// DefaultParams is the baseline policy.
func DefaultParams() Params {
	return Params{
		PriceMult:      1.30,
		CostMult:       1.00,
		CostByGig:      0,
		CostSpread:     0.33,
		OpenByGig:      0,
		OpenShare:      0.83,
		MidShare:       1.19,
		CostGain:       2.0,
		CostMin:        1.10,
		CostMax:        1.70,
		FloorMult:      1.10,
		SlowOptions:    0,
		SameDayPremium: 1.00,
		MinMarginCents: 300,
		MinMarginShare: 0.05,
		MinSlackMin:    120,
		MinSupply:      0,
		MinExperienced: 0,
		SplitTradeIns:  0,
		RiskSlope:      0,
		Premium:        1,
		TradeIns:       1,
		RiskModel:      2,
		FridayCost:     1,
		CostToday:      1,
		CostLater:      1,
		CostLong:       1,
		CostShort:      1,
		RiskMax:        1,
		RiskCost:       0,
		RiskThursday:   1,
		RiskFriday:     1,
		GuardFull:      0.955,
		GuardFloor:     0.93,
		GuardMin:       0.15,
		LoosenFrom:     0.965,
		LoosenFull:     0.985,
		LoosenRisk:     1,
		LoosenCost:     0,
		DeclineMult:    4.0,
		MarketStart:    0,
		MarketModel:    0,
		BeliefScale:    0.07,
		BeliefWorld:    0.08,
		BeliefDay:      0.07,
		BeliefDealer:   0.04,
		BeliefPlace:    0.04,
		BeliefDrift:    0.02,
		MarketScale:    0.07,
		MarketRate:     0.01,
		DealerRate:     0.01,
		DayRate:        0,
		LaneWeight:     0,
		Adaptive:       1,
		TrustBoost:     1.0,
		TrustInvest:    0,
		ShareCap:       1.0,
		ShareMarkup:    0.06,
		TrustRiskCut:   0,
		TrustFeeBoost:  1.0,
		TrustGuard:     1.0,
		TrustWinShare:  0.4,
		MinWinChance:   0.02,
		FeeStart:       0.60,
		FeeStep:        0.05,
		StepMin:        15,
		FeeMid:         1.00,
		FeeMax:         2.00,
		LeadMin:        240,
		HoldMax:        0,
		HoldSteps:      6,
		LeadDrive:      0,
		FinalMin:       45,
		UrgentSteps:    6,
		LateStart:      1.00,
		LateMax:        1.60,
		LateRampMin:    480,
		LastMult:       4.0,
		LastPriceShare: 2.0,
		OnTimePriceCap: 3.0,
		LatePriceCap:   2.0,
		LastLeadMin:    600,
		BufferShare:    0.15,
		OutagePrior:    0.05,
		TrustRushMin:   120,
		BufferMin:      15,
		Chains:         0,
		ChainTries:     1,
		ChainMaxCars:   2,
		ChainHopMin:    70,
		ChainSlackMin:  180,
		ChainTryMin:    90,
		ChainFeeShare:  0.9,
		ChainOffers:    0,
		ChainOfferMult: 0.85,
		Offers:         0,
		OfferViewers:   0,
		OfferViewMin:   120,
		OfferAfterMin:  45,
		OfferReach:     40,
		OfferMult:      1.0,
		OfferMaxScore:  600,
		Notes:          1,
		NoteHours:      0,
		RetryAfterMin:  120,
	}
}

// Levels where the defaults change: quotes from 4, the driver shortage from 7.
const (
	paramsQuoteLevel    = 4
	paramsShortageLevel = 7
)

// DefaultParamsFor is the baseline policy tuned for a level.
func DefaultParamsFor(level int) Params {
	p := DefaultParams()
	switch {
	case level < paramsQuoteLevel:
		// Every quote wins at the list price: take what pays, drivers are plentiful. What
		// does not pay is a long drive with a few hours left: it finds its driver late and
		// at a high fee (bench, level 1: +25 % margin, worst week 89.6 % -> 95.7 % on time).
		p.CostMult = 0.8
		p.MinLeadMin, p.MinLeadDrive = 180, 1
		if level >= levelSchedules {
			// Drivers learn how far our fees rise and wait for it: open near the fee
			// where gigs book and rise slowly.
			p.FeeStart, p.FeeStep = 0.75, 0.02
		}
		if level == levelSchedules {
			// Level 2 holds at 0.9x before the deadline ramp (+3 % with the lead rule, +5 %
			// without it); at levels 1 and 3 the hold at the typical fee earns more.
			p.FeeMid = 0.9
		}
		if level >= levelNetwork {
			// Gigs with several cars: chains of two orders, and a trade-in as two gigs. With
			// drivers enough, an order whose chain found nobody is worth a second attempt.
			p.Chains, p.SplitTradeIns, p.ChainTries = 1, 1, 2
		}
	case level < paramsShortageLevel:
		// A competitor near the list price and drivers enough: the margin is thin, so the
		// price may go down to a floor near the typical fee, and what is saved on fees
		// (slow clock, chains, no tight long drives) is most of the margin.
		p.FeeStart, p.FeeStep = 0.75, 0.02
		p.MinLeadMin, p.MinLeadDrive = 180, 1
		p.Chains, p.SplitTradeIns = 1, 1
		p.FloorMult, p.CostMult = 0.85, 0.92
	case level >= paramsShortageLevel:
		// A third of the drivers: the typical fee often finds nobody, and every order
		// taken uses up scarce driver time, so the cost of an order is well above its fee.
		p.CostMult = 1.3
		p.FeeStart = 0.8
		p.FeeMid = 1.15
		p.RiskMax = 0.35
		p.RiskCost = 0.5
		p.Premium = 0
		p.TradeIns = 0
		p.NoteHours = 1
		p.DayRate = 0.015
		p.Chains = 1
		p.RiskThursday = 0.8
		p.RiskFriday = 0.6
		// The belief model of the market and the lane risk model (e15). The lane model
		// knows which orders arrive late in a shortage: long hauls won with little time,
		// same-day cars on a Friday or late in a busy day, hops too short to pay a
		// driver's trip. With it the engine wins a third more orders and is as often on
		// time. Its output is a calibrated chance: the cap is lower, and the Thursday
		// and Friday factors are in the model.
		p.MarketModel = marketModelBeliefs
		p.RiskModel = riskModelLaneID
		p.RiskThursday, p.RiskFriday = 1, 1
		if level > paramsShortageLevel {
			// Level 8, judged on 24 worlds (the 8 bench weeks and 16 fresh ones): 16,127 a
			// week against 14,006 for e14, 96.9 % on time, worst week 94.1 % (e14: 92.7 %).
			// Caps of 0.16 and 0.2 earn the same within noise and are less safe. LaneWeight
			// and DestRate belong to the running-rate market model and no longer act.
			p.LaneWeight = 1
			p.DestRate = 0.02
			p.CostMult = 1.2
			p.RiskSlope = 1
			p.RiskMax = 0.12
			// A world joined on Thursday (a test final): 4,899 a week against 4,297 with
			// the full week's cost on five worlds, 162 orders against 133, both 98.8 % on
			// time. On full weeks 1.1 earns the same and is less safe, so it stays there.
			p.JoinCost = 1.1
			// The fee clock (e18). A driver looks at a gig once, so the fee shown at that
			// moment is what counts, and most bookings come in the first hours at 0.8-1.05x:
			// the hold at 1.15x mostly paid more to drivers who came anyway. The fee now
			// holds at 1.05x and creeps to 1.25x by the time the deadline pressure starts,
			// and the fee of a car that is already late no longer rises to 1.6x (its refund
			// is owed and a higher fee did not bring it in sooner). On the 24 worlds: +419 a
			// week (s.e. 137, better in 16 of them), 97.0 % on time against 96.9 %, worst
			// week 94.7 % against 94.1 %; joined on Thursday it earns the same (4,964).
			// e19: the hold starts at 1.0x, not 1.05x: +226 more (s.e. 96, better in 17),
			// 97.1 % on time, worst week 94.8 %. A start of 0.95x, a rise to 1.35x, an
			// opening fee of 0.75x and a slower clock (20 minutes a step) all earn less.
			p.FeeMid, p.HoldMax = 1.0, 1.25
			// e20: two changes that each earned a little on the 24 worlds. A request that
			// would share a driver with a car already held is priced on a lower cost, by a
			// quarter of the chance that the chain is booked: +166 a week (s.e. 137), 24
			// more orders, 0.2 points less on time. And a car of a dealer that sends 8 % of
			// the requests or more is posted to arrive on the day it is ready, for its
			// trust: +73 (s.e. 92), 0.2 points more on time.
			p.ChainCostGain = 0.25
			p.TrustRush = 0.08
			p.LateMax = 1
		}
		if level == paramsShortageLevel {
			// Level 7's competitor is only 12-38 % above the list price, so the cost decides
			// how many orders are won. On 16 fresh worlds a cost of 1.2 earns 10,551 a week
			// at 97.1 % on time (worst week 95.2 %, 356 orders); 1.1 looked best on the eight
			// bench weeks but takes 424 orders there for 10,314 at 96.4 % (worst 92.7 %), and
			// in a ranking round it took up to 522 a week and one week ended at 90.0 %.
			p.CostMult = 1.2
			p.RiskMax = 0.15
		}
	}
	return p
}

// WithOverrides returns a copy of p with the named fields (JSON tags) replaced.
func (p Params) WithOverrides(overrides map[string]string) (Params, error) {
	if len(overrides) == 0 {
		return p, nil
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return p, fmt.Errorf("encode params: %w", err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return p, fmt.Errorf("decode params: %w", err)
	}
	for name, text := range overrides {
		if _, known := fields[name]; !known {
			return p, fmt.Errorf("unknown param %q", name)
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return p, fmt.Errorf("param %s=%q is not a number", name, text)
		}
		fields[name] = value
	}
	merged, err := json.Marshal(fields)
	if err != nil {
		return p, fmt.Errorf("encode params: %w", err)
	}
	out := p
	if err := json.Unmarshal(merged, &out); err != nil {
		return p, fmt.Errorf("apply params: %w", err)
	}
	return out, nil
}

// ParamsFromMap rebuilds the params of a logged run on top of base (params the run did
// not know keep their base value).
func ParamsFromMap(base Params, logged map[string]any) (Params, error) {
	raw, err := json.Marshal(logged)
	if err != nil {
		return base, fmt.Errorf("encode logged params: %w", err)
	}
	out := base
	if err := json.Unmarshal(raw, &out); err != nil {
		return base, fmt.Errorf("decode logged params: %w", err)
	}
	return out, nil
}

// AsMap renders the params for the run log.
func (p Params) AsMap() map[string]any {
	out := map[string]any{}
	raw, err := json.Marshal(p)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}
