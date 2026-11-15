package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e25"
	VersionNote = "e24 with a lower fee-clock cap on lanes where drivers book at once and more caution in worlds with few drivers, at level 8: the clock is capped at 0.87 of the lane's fee on easy lanes and 0.94 on middle ones (110 lanes, laneQuick), the quote's cost on easy lanes at 0.95 (easyCap, midCap, easyPrice); the lower caps are off for a car under deadline pressure or late and while a tenth of the orders have never found a driver (rescueFrom 0.10); in a world under 720 drivers the modelled lateness risk counts 1.25 times (thinRoster, thinRisk); new switch, off: openScale"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
