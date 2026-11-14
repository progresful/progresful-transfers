package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"highwaycoins/internal/engine"
	"highwaycoins/internal/runner"
	"highwaycoins/internal/store"
)

// kvFlag collects repeated name=value flags.
type kvFlag map[string]string

func (k kvFlag) String() string { return fmt.Sprint(map[string]string(k)) }

func (k kvFlag) Set(text string) error {
	name, value, ok := strings.Cut(text, "=")
	if !ok || name == "" {
		return fmt.Errorf("want name=value, got %q", text)
	}
	k[name] = value
	return nil
}

func cmdRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	level := fs.Int("level", 1, "level 1-8")
	seed := fs.String("seed", "", "seed of the practice world (empty: random)")
	board := fs.Bool("board", false, "play the next ranking run at the level (counts in the practice rankings)")
	label := fs.String("label", "dev", "policy label for the run index")
	note := fs.String("note", "", "free text for the run index")
	verbose := fs.Bool("v", false, "print the engine's decisions")
	root := fs.String("root", defaultRoot, "folder of run logs")
	overrides := kvFlag{}
	fs.Var(overrides, "p", "param override name=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *board && *seed != "" {
		return fmt.Errorf("a ranking run takes no seed")
	}
	params, err := engine.DefaultParamsFor(*level).WithOverrides(overrides)
	if err != nil {
		return err
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	opt := runner.Options{Root: *root, Level: *level, Seed: *seed, Board: *board, Params: params, Label: *label, Note: *note, Out: os.Stdout}
	if *verbose {
		opt.Trace = os.Stdout
	}
	summary, err := runner.Practice(ctx, s.client, s.m, opt)
	if summary.RunID != "" {
		printSummary(summary)
	}
	return err
}

func printSummary(s store.Summary) {
	fmt.Printf("End        %s\n", runner.ScoreLine(&s.Score))
	ranked := "RANKED"
	if !s.Score.Ranked {
		ranked = "not ranked"
	}
	fmt.Printf("%s · run %s · world %s · %.0fs · %d requests\n", ranked, s.RunID, s.WorldID, s.Seconds, s.Requests)
	keys := make([]string, 0, len(s.Stats))
	for k := range s.Stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, s.Stats[k]))
	}
	fmt.Println(strings.Join(parts, " "))
}
