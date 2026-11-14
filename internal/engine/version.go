package engine

// Version names the engine's decision logic. It is bumped with every change that can
// alter what the engine decides (not for refactors), together with VersionNote. Every
// run records both, so a result can always be traced to the logic that produced it.
const (
	Version     = "e11"
	VersionNote = "level 8 as e10 but a car whose real site hours fit no single day is declined and its pickups always carry a real-hours window; levels 1-7 retuned: no long drive with a few hours left (lead rule, levels 1-6), slow fee clock from level 2, chains and split trade-ins from level 3, lower price floor at levels 4-6, cost 1.22 at level 7"
)

// SourceHash identifies the exact source the binary was built from. scripts/build.sh
// sets it and keeps a snapshot of that source under runs/engines/, so an older logic
// can be rebuilt. A binary built any other way reports "dev".
var SourceHash = "dev"
