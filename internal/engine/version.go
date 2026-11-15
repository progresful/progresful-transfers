package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e23"
	VersionNote = "e22 with fee clocks that start over at level 8: a single order's gig that its driver gives back is posted again from the opening fee when it has six hours of room (dropFreshMin 360), a chain that is given back gets a new attempt of 90 minutes (chainRecoverMin 90), a gig or chain made between 22:00 and 07:00 starts its clock in the morning (nightClock, chainNight), the slow rise of a waiting gig's fee counts 07:00 to 22:00 only (holdAwake), and a post refused because a driver took the car's earlier gig in the same minute no longer marks the car for the top fee (raceSafe); +515 a week over e22 on 96 worlds without raceSafe (s.e. 78, better in 74), on time +0.2 points; new switches, all off: morningFreshMin, stuckFrom/stuckFull/stuckMin"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
