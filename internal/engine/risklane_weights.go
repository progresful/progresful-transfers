package engine

// Weights of the lane risk model (RiskModel 4), written by the fit: do not edit by hand.
// Fitted on 98329 single-car orders of 227 level 8 practice weeks in 25 worlds; leaving
// whole worlds out, its AUC is 0.8316. The order is that of laneFeatures (risklane.go).
const riskLaneBias = +1.3345

var riskLaneWeights = [laneFeatureCount]float64{
	-0.1107, // lallP30
	-0.1369, // lallD30
	-0.2223, // lallP80
	-0.0980, // lallD80
	-0.0195, // slackH
	+0.3123, // tight
	+0.9964, // today
	-0.2290, // privP
	+1.0304, // privD
	+0.4464, // driveH
	-0.1088, // lbacklog
	+0.4623, // backlogPer
	+0.5451, // day3
	-0.6108, // day4
	-0.4379, // hour
	+0.0229, // afternoon
	+0.5840, // sameDay
	-1.7787, // threeDays
	+0.1954, // todayAft
	-1.0706, // lfee
	-2.2500, // refList
	+0.0154, // short
	+0.9353, // long
	-0.0127, // leadH
	-0.2671, // lleadH
	-0.0025, // leadPerDrive
	+0.1200, // lwonToday
	+0.6364, // lopenAll
	-0.1262, // lwon2h
	-1.2837, // todaySame
	+1.8126, // sdFri
	-0.0973, // sdThu
	-0.8021, // sdHour
	+0.0061, // sdLeadPer
	-0.3874, // sdLLead
	+0.4975, // sdDrive
	-0.1835, // sdWon
	+0.6471, // sdOpen
}
