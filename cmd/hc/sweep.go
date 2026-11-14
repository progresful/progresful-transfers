package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"sync"

	"highwaycoins/internal/engine"
	"highwaycoins/internal/runner"
	"highwaycoins/internal/store"
)

// maxParallel is the team's limit of unfinished practice worlds.
const maxParallel = 8

// combo is one setting of the swept params.
type combo map[string]string

func (c combo) String() string {
	if len(c) == 0 {
		return "(defaults)"
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + c[k]
	}
	return strings.Join(parts, " ")
}

// varied adds one setting per value of each -vary param: the fixed overrides with that one
// param changed. With the baseline (the fixed overrides alone) it shows which params matter.
func varied(fixed kvFlag, vary kvFlag) []combo {
	base := combo{}
	for k, v := range fixed {
		base[k] = v
	}
	combos := []combo{base}
	names := make([]string, 0, len(vary))
	for name := range vary {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, value := range strings.Split(vary[name], ",") {
			c := combo{name: strings.TrimSpace(value)}
			for k, v := range fixed {
				if k != name {
					c[k] = v
				}
			}
			combos = append(combos, c)
		}
	}
	return combos
}

// expand turns name=v1,v2 grids into every combination, on top of the fixed overrides.
func expand(fixed kvFlag, grids kvFlag) []combo {
	combos := []combo{{}}
	for k, v := range fixed {
		combos[0][k] = v
	}
	names := make([]string, 0, len(grids))
	for name := range grids {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var next []combo
		for _, base := range combos {
			for _, value := range strings.Split(grids[name], ",") {
				c := combo{name: strings.TrimSpace(value)}
				for k, v := range base {
					if k != name {
						c[k] = v
					}
				}
				next = append(next, c)
			}
		}
		combos = next
	}
	return combos
}

type job struct {
	combo combo
	seed  string
}

type result struct {
	job     job
	summary store.Summary
	err     error
}

func cmdSweep(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sweep", flag.ContinueOnError)
	level := fs.Int("level", 1, "level 1-8")
	seeds := fs.String("seeds", "s1,s2,s3", "comma-separated seeds; every setting plays each")
	parallel := fs.Int("parallel", 6, "worlds played at once (the team holds at most 8 unfinished)")
	label := fs.String("label", "sweep", "policy label for the run index")
	root := fs.String("root", defaultRoot, "folder of run logs")
	from := fs.Int("from", 0, "world minute at which the system starts to act (4320: Thursday 00:00, like a test final)")
	fixed, grids, vary := kvFlag{}, kvFlag{}, kvFlag{}
	fs.Var(fixed, "p", "param override name=value for every run (repeatable)")
	fs.Var(grids, "grid", "param grid name=v1,v2,... (repeatable; all combinations are played)")
	fs.Var(vary, "vary", "param name=v1,v2,... changed one at a time around the -p settings (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *parallel < 1 || *parallel > maxParallel {
		return fmt.Errorf("-parallel must be 1 to %d", maxParallel)
	}
	combos := expand(fixed, grids)
	if len(vary) > 0 {
		if len(grids) > 0 {
			return fmt.Errorf("use -grid or -vary, not both")
		}
		combos = varied(fixed, vary)
	}
	for _, c := range combos {
		if _, err := engine.DefaultParamsFor(*level).WithOverrides(c); err != nil {
			return err
		}
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	var jobs []job
	for _, c := range combos {
		for _, seed := range strings.Split(*seeds, ",") {
			jobs = append(jobs, job{combo: c, seed: strings.TrimSpace(seed)})
		}
	}
	fmt.Printf("sweep: level %d, %d settings x %d seeds = %d runs, %d at once\n", *level, len(combos), len(jobs)/len(combos), len(jobs), *parallel)
	results := runJobs(ctx, s, jobs, *parallel, func(j job) runner.Options {
		params, _ := engine.DefaultParamsFor(*level).WithOverrides(j.combo)
		return runner.Options{Root: *root, Level: *level, Seed: j.seed, Params: params, Label: *label, Note: j.combo.String(), From: *from}
	})
	printSweep(combos, results)
	return nil
}

func runJobs(ctx context.Context, s *session, jobs []job, parallel int, options func(job) runner.Options) []result {
	results := make([]result, len(jobs))
	slots := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			summary, err := runner.Practice(ctx, s.client, s.m, options(j))
			results[i] = result{job: j, summary: summary, err: err}
			status := runner.ScoreLine(&summary.Score)
			if err != nil {
				status = "ERROR " + err.Error()
			}
			fmt.Printf("  [%d/%d] %-10s %-40s %s\n", i+1, len(jobs), j.seed, j.combo, status)
		}()
	}
	wg.Wait()
	return results
}

// printSweep shows one line per setting: mean margin over the seeds, worst on-time share,
// and the difference to the first setting (the baseline) on the seeds both played, with
// the number of seeds on which the setting is ahead. A difference that does not hold on
// most seeds is noise: the world reacts chaotically to small changes.
func printSweep(combos []combo, results []result) {
	type row struct {
		name                  string
		margin, onTime, worst float64
		orders, ranked, runs  int
		bySeed                map[string]int
	}
	rows := make([]row, 0, len(combos))
	for _, c := range combos {
		r := row{name: c.String(), worst: 1, bySeed: map[string]int{}}
		for _, res := range results {
			if res.err != nil || res.job.combo.String() != r.name {
				continue
			}
			sc := res.summary.Score
			r.runs++
			r.margin += float64(sc.MarginCents) / 100
			r.onTime += sc.OnTimeShare
			r.orders += sc.WonOrders
			r.bySeed[res.job.seed] = sc.MarginCents
			if sc.OnTimeShare < r.worst {
				r.worst = sc.OnTimeShare
			}
			if sc.Ranked {
				r.ranked++
			}
		}
		if r.runs > 0 {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		fmt.Println("no run finished")
		return
	}
	base := rows[0]
	versus := func(r row) string {
		sum, ahead, n := 0.0, 0, 0
		for seed, cents := range r.bySeed {
			if b, ok := base.bySeed[seed]; ok {
				sum += float64(cents-b) / 100
				n++
				if cents > b {
					ahead++
				}
			}
		}
		if r.name == base.name || n == 0 {
			return "baseline"
		}
		return fmt.Sprintf("%+.0f (%d/%d)", sum/float64(n), ahead, n)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].margin/float64(rows[i].runs) > rows[j].margin/float64(rows[j].runs)
	})
	fmt.Printf("\n%-52s %12s %8s %8s %7s %7s %15s\n", "setting", "mean margin", "on time", "worst", "orders", "ranked", "vs first (seeds)")
	for _, r := range rows {
		n := float64(r.runs)
		fmt.Printf("%-52s %12.2f %7.1f%% %7.1f%% %7.0f %4d/%d %15s\n", r.name, r.margin/n, 100*r.onTime/n, 100*r.worst, float64(r.orders)/n, r.ranked, r.runs, versus(r))
	}
}
