package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e16"
	VersionNote = "e15 with a cost of 1.2 at level 7 (1.1 took too many orders in rich weeks); as e15: levels 7 and 8 with the belief model of the market (Gaussian beliefs about world, day, dealer and place parts of the price level, updated by every answer) and the lane risk model (lateness chance from lane length, lead, weekday and hour of the answer, same-day and load terms; risk cap 0.12 at level 8, 0.15 at level 7); levels 1-6 as e14"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
