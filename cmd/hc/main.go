// Command hc plays Spread worlds with the highway-coins engine and keeps track of the runs.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/config"
	"progresful-transfers/internal/geo"
)

const (
	envFile     = ".env"
	defaultRoot = "runs"
	usage       = `hc: Spread dispatcher

  hc run    -level N [-seed S] [-board] [-p name=value ...] [-label L] [-v]   play one practice world
  hc sweep  -level N -seeds a,b,c [-grid name=v1,v2 ...] [-parallel 6]        compare settings on the same seeds
            (or -vary name=v1,v2 ... to change one param at a time around the -p settings)
  hc bench  [-level 8]                                                        judge this engine version: default params on the fixed seeds
  hc versions [-level 8]                                                      engine versions compared on the same seeds, with the champion
  hc best   [-level 8] [-seeds a,b,c] [-n 15]                                 every setting ever tried, ranked by its average over the same seeds
  hc runs   [-n 30] [-level N] [-label L]                                     list logged runs
  hc board  [-level 8] [-n 15]                                                practice rankings
  hc worlds                                                                   the team's worlds on the server
  hc final  [-p name=value ...] [-v]                                          play the final / a test final from the live clock
  hc watch  [-team ID]                                                        record the public live feed of a (test) final: every team, every 2 s
  hc ui     [-addr 127.0.0.1:8790] [-team ID]                                  web view: live map of the run being played and replay of any logged run
  hc rehearse -level 8 [-seed S] [-world ID] [-step 2] [-tick 100ms]          play a practice world through the live code path
  hc replay [-timing] [-p name=value ...] <run folder>                         replay a logged run offline from its HTTP tape

Build with scripts/build.sh: it stamps the binary with the hash of its source and keeps a
snapshot of that source, so every run can be traced to the logic that played it.
Sweeps explore settings (many are worse on purpose); only bench runs judge an engine version.

Every run is logged under runs/<run id>/: meta.json (params), http.jsonl.gz (every request and
response with response times), events.jsonl, actions.jsonl, orders.jsonl, reveal.json, summary.json.

The team key is read from SPREAD_KEY or .env; it is never printed.`
)

// session is what every command that talks to the server needs.
type session struct {
	client *api.Client
	m      *geo.Map
}

func connect(ctx context.Context) (*session, error) {
	cfg, err := config.Load(envFile)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(cfg.URL, cfg.Key)
	if err != nil {
		return nil, err
	}
	raw, err := client.Map(ctx)
	if err != nil {
		return nil, fmt.Errorf("read map: %w", err)
	}
	saveMap(defaultRoot, raw)
	m, err := geo.New(raw)
	if err != nil {
		return nil, err
	}
	return &session{client: client, m: m}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	commands := map[string]func(context.Context, []string) error{
		"run":      cmdRun,
		"sweep":    cmdSweep,
		"runs":     cmdRuns,
		"board":    cmdBoard,
		"worlds":   cmdWorlds,
		"final":    cmdFinal,
		"rehearse": cmdRehearse,
		"replay":   cmdReplay,
		"bench":    cmdBench,
		"versions": cmdVersions,
		"best":     cmdBest,
		"watch":    cmdWatch,
		"ui":       cmdUI,
	}
	cmd, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err := cmd(ctx, os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "hc:", err)
		os.Exit(1)
	}
}
