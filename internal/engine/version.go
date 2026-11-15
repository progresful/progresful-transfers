package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e24"
	VersionNote = "e23 with the lane's fee in place of the noise of a request's typical fee at level 8: when the typical fee is above 1.04 times the lane's own fee (589 lanes of 96 practice weeks, times the world's factor learned from the first twenty requests), the fee clock, the quote's cost and the lateness model all take the capped fee (refCap 1.04, refCapPrice, refCapRisk); +639 a week over e23 on 96 worlds, 48 of them held out (s.e. 75, better in 78), on time -0.1 points, no week under 90%; new switches, all off: refCapClock, guardGraceMin, riskMonday, returnReprice"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
