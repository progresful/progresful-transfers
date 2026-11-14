package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"highwaycoins/internal/engine"
	"highwaycoins/internal/store"
)

func cmdRuns(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	n := fs.Int("n", 30, "show the last n runs")
	level := fs.Int("level", 0, "only this level (0: all)")
	label := fs.String("label", "", "only runs whose label contains this text")
	root := fs.String("root", defaultRoot, "folder of run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	all, err := store.Index(*root)
	if err != nil {
		return err
	}
	var shown []store.Summary
	for _, s := range all {
		if (*level == 0 || s.Level == *level) && strings.Contains(s.Policy, *label) {
			shown = append(shown, s)
		}
	}
	if len(shown) > *n {
		shown = shown[len(shown)-*n:]
	}
	fmt.Printf("%-11s %-2s %-12s %-10s %-16s %11s %11s %6s %7s %6s %-6s %5s  %s\n",
		"started", "L", "seed", "label", "engine", "margin", "revenue", "orders", "on time", "undel", "ranked", "secs", "settings")
	for _, s := range shown {
		ranked := "yes"
		switch {
		case s.Error != "":
			ranked = "ERROR"
		case !s.Score.Ranked:
			ranked = "no"
		}
		seed := s.Seed
		if s.Kind == "board" {
			seed = "(board)"
		}
		engineTag := "-"
		if s.Engine != "" {
			engineTag = s.Engine + " " + clip(s.EngineHash, 8)
		}
		fmt.Printf("%-11s %-2d %-12s %-10s %-16s %11.2f %11.2f %6d %6.1f%% %5.1f%% %-6s %5.0f  %s\n",
			s.StartedAt.Local().Format("01-02 15:04"), s.Level, clip(seed, 12), clip(s.Policy, 10), engineTag,
			float64(s.Score.MarginCents)/100, float64(s.Score.RevenueCents)/100, s.Score.WonOrders,
			100*s.Score.OnTimeShare, 100*s.Score.UndeliveredShare, ranked, s.Seconds, changedParams(s.Level, s.Params))
	}
	fmt.Printf("%d of %d runs\n", len(shown), len(all))
	return nil
}

func clip(text string, width int) string {
	if len(text) <= width {
		return text
	}
	return text[:width-1] + "~"
}

// changedParams lists the params of a run that differ from the current defaults of its level.
func changedParams(level int, params map[string]any) string {
	defaults := engine.DefaultParamsFor(level).AsMap()
	var parts []string
	for name, value := range params {
		if fmt.Sprint(defaults[name]) != fmt.Sprint(value) {
			parts = append(parts, fmt.Sprintf("%s=%v", name, value))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func cmdBoard(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	level := fs.Int("level", 8, "level 1-8")
	n := fs.Int("n", 15, "teams to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	board, err := s.client.Board(ctx, *level)
	if err != nil {
		return err
	}
	if board.Current == nil {
		fmt.Println("no ranking round yet")
		return nil
	}
	season := board.Current
	fmt.Printf("level %d · round %d %q · %d teams\n", *level, season.Season, season.Name, len(season.Teams))
	for i, t := range season.Teams {
		if i >= *n {
			break
		}
		rank, margin, onTime := "-", "      -", "    -"
		if t.Rank != nil {
			rank = fmt.Sprint(*t.Rank)
		}
		if t.MarginCents != nil {
			margin = fmt.Sprintf("%10.2f", float64(*t.MarginCents)/100)
		}
		if t.OnTimeShare != nil {
			onTime = fmt.Sprintf("%5.1f%%", 100**t.OnTimeShare)
		}
		fmt.Printf("%3s  %-22s played %d  margin %s  on time %s  ranked %v\n", rank, clip(t.Team, 22), t.Played, margin, onTime, t.Ranked)
	}
	return nil
}

func cmdWorlds(ctx context.Context, _ []string) error {
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	worlds, err := s.client.Worlds(ctx)
	if err != nil {
		return err
	}
	open := 0
	for _, w := range worlds {
		if !w.Ended {
			open++
			fmt.Printf("OPEN  %s  %s level %d\n", w.WorldID, w.Kind, w.Level)
		}
	}
	fmt.Printf("%d worlds, %d not ended (limit 8)\n", len(worlds), open)
	return nil
}
