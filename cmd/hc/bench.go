package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"progresful-transfers/internal/engine"
	"progresful-transfers/internal/runner"
	"progresful-transfers/internal/store"
)

const (
	// benchLabel marks the runs that judge an engine version: the level's default
	// params on the fixed seeds, nothing swept.
	benchLabel = "bench"
	// benchSeeds is the fixed suite every engine version plays, so versions are compared
	// on the same weeks. Do not change it: a new suite makes old results incomparable.
	benchSeeds = "spread-1,n2,n4,n5,b1,b2,b3,b4"
	devHash    = "dev"
	noteWidth  = 70
)

// cmdBench plays the fixed seed suite with the engine's default params for a level and
// then shows how this engine version compares with every earlier one on the same seeds.
func cmdBench(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	level := fs.Int("level", 8, "level 1-8")
	seeds := fs.String("seeds", benchSeeds, "comma-separated seeds (keep the default to compare versions)")
	parallel := fs.Int("parallel", maxParallel, "worlds played at once")
	root := fs.String("root", defaultRoot, "folder of run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if engine.SourceHash == devHash {
		fmt.Println("warning: this binary was not built with scripts/build.sh, so its runs are stamped \"dev\" and cannot be traced to a source snapshot")
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	var jobs []job
	for _, seed := range strings.Split(*seeds, ",") {
		jobs = append(jobs, job{combo: combo{}, seed: strings.TrimSpace(seed)})
	}
	fmt.Printf("bench: engine %s (%s), level %d, %d seeds\n", engine.Version, engine.SourceHash, *level, len(jobs))
	runJobs(ctx, s, jobs, *parallel, func(j job) runner.Options {
		return runner.Options{Root: *root, Level: *level, Seed: j.seed, Params: engine.DefaultParamsFor(*level), Label: benchLabel}
	})
	return printVersions(*root, *level)
}

func cmdVersions(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("versions", flag.ContinueOnError)
	level := fs.Int("level", 8, "level 1-8")
	root := fs.String("root", defaultRoot, "folder of run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return printVersions(*root, *level)
}

// versionResult is the bench result of one engine version at one level.
type versionResult struct {
	engine, hash, note string
	first              store.Summary
	bySeed             map[string]store.Summary
}

func (v *versionResult) mean(seeds []string) (margin, onTime, worst float64, ranked int) {
	worst = 1
	for _, seed := range seeds {
		sc := v.bySeed[seed].Score
		margin += float64(sc.MarginCents) / 100
		onTime += sc.OnTimeShare
		worst = min(worst, sc.OnTimeShare)
		if sc.Ranked {
			ranked++
		}
	}
	n := float64(max(1, len(seeds)))
	return margin / n, onTime / n, worst, ranked
}

func (v *versionResult) seeds() []string {
	out := make([]string, 0, len(v.bySeed))
	for seed := range v.bySeed {
		out = append(out, seed)
	}
	sort.Strings(out)
	return out
}

// benchResults groups the bench runs of a level by engine version, oldest first.
func benchResults(root string, level int) ([]*versionResult, error) {
	all, err := store.Index(root)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*versionResult{}
	var order []*versionResult
	for _, s := range all {
		if s.Policy != benchLabel || s.Level != level || s.Error != "" || !s.WorldEnded {
			continue
		}
		key := s.Engine + "|" + s.EngineHash
		v := byKey[key]
		if v == nil {
			v = &versionResult{engine: s.Engine, hash: s.EngineHash, note: s.EngineNote, first: s, bySeed: map[string]store.Summary{}}
			byKey[key] = v
			order = append(order, v)
		}
		v.bySeed[s.Seed] = s
	}
	return order, nil
}

// champion is the version with the highest mean margin among those that played the
// largest suite and were ranked on every seed of it.
func champion(versions []*versionResult) *versionResult {
	most := 0
	for _, v := range versions {
		most = max(most, len(v.bySeed))
	}
	var best *versionResult
	bestMargin := 0.0
	for _, v := range versions {
		seeds := v.seeds()
		margin, _, _, ranked := v.mean(seeds)
		if len(seeds) < most || ranked < len(seeds) {
			continue
		}
		if best == nil || margin > bestMargin {
			best, bestMargin = v, margin
		}
	}
	return best
}

// printVersions shows every benched engine version of a level on the same seeds, with
// its difference to the champion seed by seed.
func printVersions(root string, level int) error {
	versions, err := benchResults(root, level)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Printf("no bench runs at level %d yet: run `hc bench -level %d`\n", level, level)
		return nil
	}
	champ := champion(versions)
	fmt.Printf("\nengine versions at level %d (bench: default params on the fixed seeds)\n", level)
	fmt.Printf("%-2s %-6s %-12s %-11s %5s %11s %8s %7s %7s %14s  %s\n", "", "engine", "source", "benched", "seeds", "mean margin", "on time", "worst", "ranked", "vs champion", "what it does")
	for _, v := range versions {
		seeds := v.seeds()
		margin, onTime, worst, ranked := v.mean(seeds)
		mark, versus := "", ""
		switch {
		case v == champ:
			mark, versus = "★", "champion"
		case champ != nil:
			versus = pairedDelta(v, champ)
		}
		fmt.Printf("%-2s %-6s %-12s %-11s %5d %11.2f %7.1f%% %6.1f%% %4d/%-2d %14s  %s\n", mark, v.engine, v.hash, v.first.StartedAt.Local().Format("01-02 15:04"),
			len(seeds), margin, 100*onTime, 100*worst, ranked, len(seeds), versus, clip(v.note, noteWidth))
	}
	if champ == nil {
		fmt.Println("no champion yet: a champion must be ranked (90% on time) on every seed of the suite")
	}
	return nil
}

// pairedDelta compares two versions on the seeds both played: mean margin difference and
// on how many seeds the first is ahead.
func pairedDelta(v, other *versionResult) string {
	sum, ahead, n := 0.0, 0, 0
	for seed, s := range v.bySeed {
		o, ok := other.bySeed[seed]
		if !ok {
			continue
		}
		diff := float64(s.Score.MarginCents-o.Score.MarginCents) / 100
		sum += diff
		n++
		if diff > 0 {
			ahead++
		}
	}
	if n == 0 {
		return "no common seed"
	}
	return fmt.Sprintf("%+.0f (%d/%d)", sum/float64(n), ahead, n)
}
