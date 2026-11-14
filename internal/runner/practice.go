// Package runner drives an engine through a world: step by step on a practice world,
// and (later) from the live stream in the final.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/engine"
	"highwaycoins/internal/geo"
	"highwaycoins/internal/store"
)

const (
	kindPractice = "practice"
	kindBoard    = "board"
	ordersFile   = "orders.jsonl"
	cleanupWait  = 15 * time.Second
	// A full set of practice slots answers 409; another run usually ends within seconds.
	slotsFullStatus = 409
	slotRetries     = 60
	slotWait        = 3 * time.Second
)

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Options describe one practice run.
type Options struct {
	Root   string // folder of run logs
	Level  int
	Seed   string // empty: the server picks one
	Board  bool   // a ranking run: counts in the practice rankings, no reveal
	Params engine.Params
	Label  string    // policy label for the run index
	Note   string    // free text for the run index
	Out    io.Writer // day-by-day progress; nil is quiet
	Trace  io.Writer // engine decisions; nil is quiet
	// From is the world minute at which the system starts to act; before it the world
	// runs without it and its requests go to the competitor. 4320 (Thursday 00:00) makes a
	// practice world like a test final, which is joined in the middle of its week.
	From int
}

// Practice creates a practice world, plays it to the end and logs the run.
func Practice(ctx context.Context, c *api.Client, m *geo.Map, opt Options) (store.Summary, error) {
	// Every request of the run goes on its tape, from the one that creates the world.
	tape := &api.Tape{}
	ctx = api.WithRecorder(ctx, tape)
	world, err := createWorld(ctx, c, opt)
	if err != nil {
		return store.Summary{}, fmt.Errorf("create world: %w", err)
	}
	started := time.Now()
	kind := kindPractice
	if opt.Board {
		kind = kindBoard
	}
	meta := store.Meta{
		RunID:     fmt.Sprintf("%s-L%d-%s-%s", started.Format("20060102-150405"), opt.Level, unsafeName.ReplaceAllString(seedName(opt), "_"), world.ID[len(world.ID)-6:]),
		StartedAt: started, WorldID: world.ID, Kind: kind, Level: opt.Level, Seed: opt.Seed,
		Policy: opt.Label, Params: opt.Params.AsMap(), Note: opt.Note,
		Engine: engine.Version, EngineHash: engine.SourceHash, EngineNote: engine.VersionNote,
	}
	run, err := store.NewRun(opt.Root, meta)
	if err != nil {
		cleanup(world, opt.Board)
		return store.Summary{}, err
	}
	tape.Attach(run.HTTP)
	eng := engine.New(opt.Level, opt.Params, m)
	if opt.Trace != nil {
		eng.Logf = func(format string, args ...any) { fmt.Fprintf(opt.Trace, format+"\n", args...) }
	}
	_, playErr := play(ctx, world, eng, run, opt.Out, opt.From)
	summary := store.Summary{Meta: meta, Stats: eng.Stats()}
	if playErr != nil {
		summary.Error = playErr.Error()
		cleanup(world, opt.Board)
	}
	return finish(context.WithValue(ctx, tapeKey{}, tape), world, eng, run, summary, opt.Board, playErr)
}

// tapeKey carries the run's tape to finish, for the request count.
type tapeKey struct{}

// createWorld makes the practice world. When the team's eight slots are full (other runs
// are still playing) it waits for one instead of failing; a ranking run that cannot
// start (409 for other reasons) is not retried.
func createWorld(ctx context.Context, c *api.Client, opt Options) (*api.World, error) {
	for attempt := 0; ; attempt++ {
		world, err := c.Create(ctx, opt.Level, opt.Seed, opt.Board)
		if err == nil || opt.Board || api.StatusOf(err) != slotsFullStatus || attempt >= slotRetries {
			return world, err
		}
		if err := sleep(ctx, slotWait); err != nil {
			return nil, err
		}
	}
}

// CreatePractice makes a practice world, waiting for one of the team's slots when all
// eight are taken (by sweeps running at the same time).
func CreatePractice(ctx context.Context, c *api.Client, level int, seed string) (*api.World, error) {
	return createWorld(ctx, c, Options{Level: level, Seed: seed})
}

func seedName(opt Options) string {
	switch {
	case opt.Board:
		return "board"
	case opt.Seed == "":
		return "random"
	}
	return opt.Seed
}

// cleanup frees the slot of a practice world that stopped on an error. A ranking run is
// left alone: deleting it would let it play to its end without the system, and count.
func cleanup(world *api.World, board bool) {
	if board {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupWait)
	defer cancel()
	_ = world.Delete(ctx)
}

// play steps the world from one event that needs an answer to the next.
func play(ctx context.Context, world *api.World, eng *engine.Engine, run *store.Run, out io.Writer, from int) (int, error) {
	if from > 0 {
		// The world's past before the system joins: logged, not acted on.
		past, err := world.AdvanceUntil(ctx, nil, from, nil)
		if err != nil {
			return 1, fmt.Errorf("run the world to minute %d: %w", from, err)
		}
		for i := range past.Events {
			run.Event(past.Events[i].Raw)
		}
	}
	snap, err := world.Snapshot(ctx)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}
	roster, err := world.Roster(ctx)
	if err != nil {
		return 0, fmt.Errorf("roster: %w", err)
	}
	if err := run.SaveRoster(roster); err != nil {
		return 0, err
	}
	eng.Begin(snap, roster)
	steps, now := 2, snap.Now
	for now < snap.EndsAt {
		sent := eng.Drain()
		wake := min(snap.EndsAt, max(now+1, eng.NextWake()))
		step, err := world.AdvanceUntil(ctx, eng.WakeKinds(), wake, actionsOf(sent))
		steps++
		if err != nil {
			return steps, err
		}
		logActions(run, now, sent, step.Results)
		day := cal.DayOf(now)
		now = step.Now
		eng.SetNow(now)
		eng.Resolve(sent, step.Results)
		for i := range step.Events {
			run.Event(step.Events[i].Raw)
		}
		eng.Handle(step.Events)
		for _, d := range eng.Decisions() {
			run.Decision(d)
		}
		if cal.DayOf(now) > day {
			// Read every day, shown or not, so a run makes the same requests either way.
			steps++
			reportDay(ctx, world, out, day)
		}
	}
	return steps, nil
}

func actionsOf(sent []engine.Pending) []api.Action {
	actions := make([]api.Action, len(sent))
	for i, p := range sent {
		actions[i] = p.Action
	}
	return actions
}

func logActions(run *store.Run, at int, sent []engine.Pending, results []api.ActionResult) {
	for i, p := range sent {
		if i < len(results) {
			run.Action(at, p.Action, results[i])
		}
	}
}

var weekdays = [7]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

func reportDay(ctx context.Context, world *api.World, out io.Writer, day int) {
	score, err := world.Score(ctx)
	if out == nil {
		return
	}
	if err != nil {
		fmt.Fprintf(out, "%-9s  score unavailable: %v\n", weekdays[day%7], err)
		return
	}
	fmt.Fprintf(out, "%-9s  %s\n", weekdays[day%7], ScoreLine(score))
}

// ScoreLine renders a score as one line.
func ScoreLine(s *api.Score) string {
	return fmt.Sprintf("margin €%10.2f  revenue €%10.2f  %4d orders  %5.1f%% on time  %4.1f%% not delivered",
		float64(s.MarginCents)/100, float64(s.RevenueCents)/100, s.WonOrders, 100*s.OnTimeShare, 100*s.UndeliveredShare)
}

// finish reads the final score and reveal, and closes the run log.
func finish(ctx context.Context, world *api.World, eng *engine.Engine, run *store.Run, s store.Summary, board bool, playErr error) (store.Summary, error) {
	var reveal json.RawMessage
	if playErr == nil {
		score, err := world.Score(ctx)
		if err != nil {
			playErr = fmt.Errorf("score: %w", err)
			s.Error = playErr.Error()
		} else {
			s.Score, s.WorldEnded = *score, true
		}
		if !board {
			// The reveal is research data; a run without it is still a run.
			if raw, err := world.RevealRaw(ctx); err == nil {
				reveal = raw
			}
		}
	}
	s.EndedAt = time.Now()
	s.Seconds = s.EndedAt.Sub(s.StartedAt).Seconds()
	if tape, ok := ctx.Value(tapeKey{}).(*api.Tape); ok {
		s.Requests = tape.Count()
	}
	lines := eng.OrderReport()
	if err := run.Lines(ordersFile, len(lines), func(i int) any { return lines[i] }); err != nil && playErr == nil {
		playErr = err
	}
	if err := run.Finish(s, reveal); err != nil && playErr == nil {
		playErr = err
	}
	return s, playErr
}
