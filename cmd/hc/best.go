package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"progresful-transfers/internal/store"
)

const (
	// historySeeds are the seeds most of the logged level 8 runs share; `hc best` averages
	// over them by default so settings from different sweeps can be compared.
	historySeeds  = "spread-1,n2,n4,n5"
	researchParam = "research"
)

// setting is one combination of engine version and params, with its runs by seed.
type setting struct {
	engine, hash, label string
	params              map[string]any
	first               store.Summary
	bySeed              map[string]store.Summary
}

// cmdBest ranks every setting ever played (sweeps included) by its average over the same
// seeds, so the best results can be traced back to the exact settings and engine version.
// A setting counts only if it played every seed asked for; ranked-on-all settings come first.
func cmdBest(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("best", flag.ContinueOnError)
	level := fs.Int("level", 8, "level 1-8")
	seedList := fs.String("seeds", historySeeds, "the seeds a setting must have played; the average is over exactly these")
	n := fs.Int("n", 15, "settings to show")
	root := fs.String("root", defaultRoot, "folder of run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	seeds := strings.Split(*seedList, ",")
	for i := range seeds {
		seeds[i] = strings.TrimSpace(seeds[i])
	}
	all, err := store.Index(*root)
	if err != nil {
		return err
	}
	settings := groupSettings(all, *level)
	type row struct {
		s                     *setting
		margin, onTime, worst float64
		ranked                int
	}
	var rows []row
	for _, s := range settings {
		r := row{s: s, worst: 1}
		complete := true
		for _, seed := range seeds {
			run, ok := s.bySeed[seed]
			if !ok {
				complete = false
				break
			}
			r.margin += float64(run.Score.MarginCents) / 100
			r.onTime += run.Score.OnTimeShare
			r.worst = min(r.worst, run.Score.OnTimeShare)
			if run.Score.Ranked {
				r.ranked++
			}
		}
		if complete {
			r.margin /= float64(len(seeds))
			r.onTime /= float64(len(seeds))
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if all1, all2 := rows[i].ranked == len(seeds), rows[j].ranked == len(seeds); all1 != all2 {
			return all1
		}
		return rows[i].margin > rows[j].margin
	})
	fmt.Printf("level %d: %d settings played all of %s (average over these %d weeks; ranked on all first)\n", *level, len(rows), *seedList, len(seeds))
	fmt.Printf("%3s %11s %8s %7s %7s  %-11s %-16s %-9s %s\n", "#", "mean margin", "on time", "worst", "ranked", "played", "engine", "label", "settings (vs today's defaults)")
	for i, r := range rows {
		if i >= *n {
			break
		}
		engineTag := "(before versions)"
		if r.s.engine != "" {
			engineTag = r.s.engine + " " + clip(r.s.hash, 8)
		}
		fmt.Printf("%3d %11.2f %7.1f%% %6.1f%% %4d/%-2d  %-11s %-16s %-9s %s\n", i+1, r.margin, 100*r.onTime, 100*r.worst, r.ranked, len(seeds),
			r.s.first.StartedAt.Local().Format("01-02 15:04"), engineTag, clip(r.s.label, 9), changedParams(*level, r.s.params))
	}
	return nil
}

// groupSettings groups finished practice runs of a level by engine version and full params.
func groupSettings(all []store.Summary, level int) []*setting {
	byKey := map[string]*setting{}
	var order []*setting
	for _, run := range all {
		if run.Level != level || run.Kind != "practice" || run.Error != "" || !run.WorldEnded {
			continue
		}
		if v, ok := run.Params[researchParam].(float64); ok && v == 1 {
			continue // study weeks post no gigs
		}
		params, err := json.Marshal(run.Params) // map keys are sorted: a stable key
		if err != nil {
			continue
		}
		key := run.Engine + "|" + run.EngineHash + "|" + string(params)
		s := byKey[key]
		if s == nil {
			s = &setting{engine: run.Engine, hash: run.EngineHash, label: run.Policy, params: run.Params, first: run, bySeed: map[string]store.Summary{}}
			byKey[key] = s
			order = append(order, s)
		}
		s.bySeed[run.Seed] = run
	}
	return order
}
