package engine

// Lane price effects: the part of the competitor's price that belongs to the pickup and
// the delivery place and holds in every world (HANDBOOK.md, "Dealers compare prices":
// the lane factor is higher where little work comes back and where few drivers live near
// the delivery place). Fitted on 566,915 of our own quote outcomes in 24 practice worlds
// as a shift of the log price level, with each world's own level and each dealer's trust
// taken out. Tested by leaving one world out at a time: the effects learned on the other
// worlds predicted wins better on all 24 (mean gain 0.042 nats per quote).

// laneEffectMean is the mean of pickup plus delivery effect over the requests of a week,
// taken out so that the world level keeps its meaning.
const laneEffectMean = 0.1817

// lanePickupEffect is the shift of the log price level by pickup place.
var lanePickupEffect = map[string]float64{
	"espoo": 0.202, "hameenlinna": 0.013, "helsinki": 0.174, "hyvinkaa": 0.220, "joensuu": 0.043,
	"jyvaskyla": 0.057, "kajaani": -0.003, "kemi": -0.020, "kokkola": -0.010, "kotka": -0.057,
	"kouvola": 0.047, "kuopio": -0.057, "lahti": 0.064, "lappeenranta": 0.047, "lempaala": 0.051,
	"mikkeli": -0.153, "oulu": 0.062, "pori": -0.012, "porvoo": 0.111, "raisio": 0.063,
	"rauma": -0.014, "rovaniemi": 0.013, "salo": 0.168, "savonlinna": -0.033, "seinajoki": 0.051,
	"tampere": 0.097, "turku": 0.028, "vaasa": 0.092, "vantaa": 0.238, "ylivieska": -0.015,
}

// laneDeliveryEffect is the shift of the log price level by delivery place.
var laneDeliveryEffect = map[string]float64{
	"espoo": 0.004, "hameenlinna": 0.090, "helsinki": -0.001, "hyvinkaa": -0.084, "joensuu": 0.139,
	"jyvaskyla": 0.030, "kajaani": 0.213, "kemi": 0.236, "kokkola": 0.198, "kotka": 0.115,
	"kouvola": 0.211, "kuopio": 0.082, "lahti": 0.135, "lappeenranta": 0.153, "lempaala": 0.041,
	"mikkeli": 0.143, "oulu": 0.086, "pori": 0.121, "porvoo": -0.093, "raisio": 0.005,
	"rauma": 0.095, "rovaniemi": 0.229, "salo": 0.059, "savonlinna": 0.152, "seinajoki": 0.173,
	"tampere": 0.055, "turku": 0.060, "vaasa": 0.149, "vantaa": -0.064, "ylivieska": 0.156,
}

// laneShift is how much the price level of a lane lies above or below the week's average
// (log ratio). Unknown places count as average.
func laneShift(pickup, delivery string) float64 {
	p, okP := lanePickupEffect[pickup]
	d, okD := laneDeliveryEffect[delivery]
	if !okP || !okD {
		return 0
	}
	return p + d - laneEffectMean
}
