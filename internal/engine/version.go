package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e17"
	VersionNote = "e16 for a full week (same decisions); in a world joined after its first day with no order of ours (a test final starts on Thursday) the cost is 1.1 instead of 1.2, and the past's requests are no longer priced anew on joining (up to e16 each taught the price model a loss that never happened); as e16: levels 7 and 8 with the belief model of the market and the lane risk model (risk cap 0.12 at level 8, 0.15 at level 7, cost 1.2), levels 1-6 as e14"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
