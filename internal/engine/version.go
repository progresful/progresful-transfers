package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e9"
	VersionNote = "e8 + chains: two single-car orders go in one gig when the second pickup is within 25 road minutes of the first " +
		"delivery and both stay on time; a 90-minute attempt at 0.9 of the two typical fees, then the cars go alone"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
