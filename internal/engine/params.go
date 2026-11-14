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
	// FloorMult is the least price as a multiple of the typical fee (level 4+): the fee
	// itself. A quote goes under CostMult only when the order is worth more than its
	// margin, for the trust it builds.
	FloorMult float64 `json:"floorMult"`
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
	// MinSupply declines a request when fewer drivers than this live within the app's
	// notification radius of the pickup or of the delivery (whichever has more). For a
	// premium car only experienced drivers count, against MinExperienced.
	MinSupply      int `json:"minSupply"`
	MinExperienced int `json:"minExperienced"`
	// TradeIns 1 quotes trade-ins (two cars, one visit at the customer's booked hour); 0
	// declines them. In the shortage almost half arrived late: a driver needs a long free
	// block around a fixed hour, and the fee ran to 1.4 times the typical one.
	TradeIns int `json:"tradeIns"`
	// Premium 1 quotes premium cars (experienced drivers only); 0 declines them. In the
	// shortage they arrived late almost one time in five and earned a third less.
	Premium int `json:"premium"`
	// RiskModel picks the lateness model: 2 the supply model (the champion's), 3 the one
	// that also knows weekday, hour and delivery option (its risks are on a lower scale:
	// use RiskMax near 0.2 and no Thursday/Friday factors with it).
	RiskModel int `json:"riskModel"`
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
	// DayRate is how fast the market level follows the current day alone (per dealer and
	// region too): the competitor's day factor, the price war and the outage. 0 turns it off.
	DayRate float64 `json:"dayRate"`
	// LaneWeight is how much of the fitted lane price effects (lanes.go) the quote uses:
	// 1 prices each lane against its own level, 0 one level for all lanes.
	LaneWeight float64 `json:"laneWeight"`
	// Adaptive 0 quotes the fixed PriceMult; 1 quotes the ratio with the best expected gain.
	Adaptive int `json:"adaptive"`
	// TrustBoost scales how much of a dealer's trust (level 8: +1% per on-time order) the
	// quote claims; TrustInvest scales the extra worth of an order that builds trust.
	TrustBoost  float64 `json:"trustBoost"`
	TrustInvest float64 `json:"trustInvest"`
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
	// Direct offers (level 3+). Offers 1 turns them on. A gig gets offers once it has been
	// open OfferAfterMin minutes, or at once when fewer than OfferReach drivers live within
	// the app's 30 km notification radius of its pickup. An offer pays at least OfferMult
	// times the base fee. OfferMaxScore bounds the travel minutes of a driver worth asking.
	Offers        int     `json:"offers"`
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
		CostGain:       2.0,
		CostMin:        1.10,
		CostMax:        1.70,
		FloorMult:      1.10,
		SameDayPremium: 1.00,
		MinMarginCents: 300,
		MinMarginShare: 0.05,
		MinSlackMin:    120,
		MinSupply:      0,
		MinExperienced: 0,
		Premium:        1,
		TradeIns:       1,
		RiskModel:      2,
		FridayCost:     1,
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
		BufferMin:      15,
		Offers:         0,
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
		// Every quote wins at the list price: take what pays, drivers are plentiful.
		p.CostMult = 0.8
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
		p.RiskThursday = 0.8
		p.RiskFriday = 0.6
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
