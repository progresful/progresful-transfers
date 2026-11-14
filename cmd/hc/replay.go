package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"highwaycoins/internal/api"
	"highwaycoins/internal/engine"
	"highwaycoins/internal/geo"
	"highwaycoins/internal/runner"
	"highwaycoins/internal/store"
)

const (
	mapCacheFile = "map.json"
	replayURL    = "http://replay.invalid"
	replayKey    = "replay"
)

// saveMap keeps the last map read from the server, so a replay needs no network.
func saveMap(root string, m *api.WorldMap) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	if os.MkdirAll(root, 0o755) == nil {
		_ = os.WriteFile(filepath.Join(root, mapCacheFile), data, 0o644)
	}
}

func loadMap(root string) (*geo.Map, error) {
	data, err := os.ReadFile(filepath.Join(root, mapCacheFile))
	if err != nil {
		return nil, fmt.Errorf("no cached map (run any online command once): %w", err)
	}
	var raw api.WorldMap
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode cached map: %w", err)
	}
	return geo.New(&raw)
}

// cmdReplay plays a logged practice run again, offline, against its tape: the recorded
// answers are served as long as the engine sends the recorded requests. A run that
// replays to the end made the same decisions; the first different request is reported.
func cmdReplay(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	root := fs.String("root", defaultRoot, "folder of run logs")
	timing := fs.Bool("timing", false, "only print the response-time summary of the tape")
	overrides := kvFlag{}
	fs.Var(overrides, "p", "param override name=value on top of the run's logged params (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: hc replay [-timing] [-p name=value ...] <run folder>")
	}
	dir := fs.Arg(0)
	meta, err := store.ReadMeta(dir)
	if err != nil {
		return err
	}
	tape, err := store.ReadTape(dir)
	if err != nil {
		return err
	}
	printTiming(tape)
	if *timing {
		return nil
	}
	if meta.Kind == "final" {
		return errors.New("a live run cannot be replayed step by step: its requests depend on the wall clock")
	}
	params, err := engine.ParamsFromMap(engine.DefaultParamsFor(meta.Level), meta.Params)
	if err != nil {
		return err
	}
	if params, err = params.WithOverrides(overrides); err != nil {
		return err
	}
	m, err := loadMap(*root)
	if err != nil {
		return err
	}
	client, err := api.NewClient(replayURL, replayKey)
	if err != nil {
		return err
	}
	transport := api.NewReplayTransport(tape)
	client.SetTransport(transport)
	scratch, err := os.MkdirTemp("", "hc-replay-")
	if err != nil {
		return fmt.Errorf("make scratch folder: %w", err)
	}
	defer os.RemoveAll(scratch)
	opt := runner.Options{Root: scratch, Level: meta.Level, Seed: meta.Seed, Board: meta.Kind == "board", Params: params, Label: "replay"}
	summary, err := runner.Practice(ctx, client, m, opt)
	var diverged *api.DivergedError
	if errors.As(err, &diverged) {
		fmt.Printf("DIVERGED after %d of %d requests: the engine no longer makes the recorded decisions.\n%v\n", diverged.N-1, len(tape), diverged)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("replayed %d of %d requests: same decisions. %s\n", len(tape)-transport.Remaining(), len(tape), runner.ScoreLine(&summary.Score))
	return nil
}

// printTiming summarises the response times on a tape, by route.
func printTiming(tape []api.Exchange) {
	byRoute := map[string][]float64{}
	for _, ex := range tape {
		route := ex.Method + " " + routeOf(ex.Path)
		byRoute[route] = append(byRoute[route], ex.Ms)
	}
	routes := make([]string, 0, len(byRoute))
	for route := range byRoute {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	fmt.Printf("%-34s %6s %8s %8s %8s %8s\n", "route", "n", "p50 ms", "p90 ms", "p99 ms", "max ms")
	for _, route := range routes {
		ms := byRoute[route]
		sort.Float64s(ms)
		q := func(p float64) float64 { return ms[int(p*float64(len(ms)-1))] }
		fmt.Printf("%-34s %6d %8.1f %8.1f %8.1f %8.1f\n", route, len(ms), q(0.5), q(0.9), q(0.99), ms[len(ms)-1])
	}
}

// routeOf strips the world id and the query from a path.
func routeOf(path string) string {
	for i := 0; i < len(path); i++ {
		if path[i] == '?' {
			path = path[:i]
			break
		}
	}
	const prefix = "/v1/worlds/"
	if len(path) > len(prefix) && path[:len(prefix)] == prefix {
		rest := path[len(prefix):]
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				return prefix + "{id}" + rest[i:]
			}
		}
		return prefix + "{id}"
	}
	return path
}
