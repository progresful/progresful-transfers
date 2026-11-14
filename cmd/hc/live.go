package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"highwaycoins/internal/engine"
	"highwaycoins/internal/runner"
)

// finalLevel is the level of the final and of the test finals.
const finalLevel = 8

// cmdFinal plays the team's final (or a test final) from the live clock.
func cmdFinal(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("final", flag.ContinueOnError)
	label := fs.String("label", "final", "policy label for the run index")
	verbose := fs.Bool("v", false, "print the engine's decisions")
	root := fs.String("root", defaultRoot, "folder of run logs")
	poll := fs.Duration("poll", 0, "how often to read new events (default 150ms)")
	overrides := kvFlag{}
	fs.Var(overrides, "p", "param override name=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	params, err := engine.DefaultParamsFor(finalLevel).WithOverrides(overrides)
	if err != nil {
		return err
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	opt := runner.LiveOptions{Root: *root, Params: params, Label: *label, Out: os.Stdout, Poll: *poll}
	if *verbose {
		opt.Trace = os.Stdout
	}
	return runner.Final(ctx, s.client, s.m, opt)
}

// cmdRehearse plays a practice world as if it were live: a ticker moves the clock while
// the live loop reads events and acts. It checks the final's code path before the final.
func cmdRehearse(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("rehearse", flag.ContinueOnError)
	level := fs.Int("level", 8, "level 1-8")
	seed := fs.String("seed", "", "seed of the practice world (empty: random)")
	worldID := fs.String("world", "", "join this practice world instead of creating one (restart test)")
	step := fs.Int("step", 2, "world minutes per tick")
	tick := fs.Duration("tick", 100*time.Millisecond, "real time per tick")
	keep := fs.Bool("keep", false, "keep the practice world when the rehearsal stops (to test a restart with -world)")
	from := fs.Int("from", 0, "run the new world to this minute without the system first (4320: Thursday 00:00, like a test final)")
	label := fs.String("label", "rehearse", "policy label for the run index")
	root := fs.String("root", defaultRoot, "folder of run logs")
	overrides := kvFlag{}
	fs.Var(overrides, "p", "param override name=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	params, err := engine.DefaultParamsFor(*level).WithOverrides(overrides)
	if err != nil {
		return err
	}
	s, err := connect(ctx)
	if err != nil {
		return err
	}
	world := s.client.World(*worldID)
	if *worldID == "" {
		if world, err = runner.CreatePractice(ctx, s.client, *level, *seed); err != nil {
			return fmt.Errorf("create world: %w", err)
		}
	}
	if *from > 0 && *worldID == "" {
		if _, err := world.AdvanceUntil(ctx, nil, *from, nil); err != nil {
			return fmt.Errorf("run the world to minute %d: %w", *from, err)
		}
	}
	fmt.Printf("rehearsal on practice world %s: %d world minutes every %s\n", world.ID, *step, *tick)
	clock := runner.SimulatedClock(ctx, world, *step, *tick)
	opt := runner.LiveOptions{Root: *root, Params: params, Label: *label, Out: os.Stdout}
	err = runner.PlayLive(ctx, s.client, world, s.m, clock, opt)
	if !*keep {
		// A practice world that has not ended holds one of the team's eight slots.
		cleanCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if derr := world.Delete(cleanCtx); derr == nil {
			fmt.Printf("practice world %s deleted\n", world.ID)
		}
	}
	return err
}
