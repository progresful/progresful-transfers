package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/engine"
	"highwaycoins/internal/geo"
	"highwaycoins/internal/store"
)

const (
	kindFinal       = "final"
	defaultPoll     = 150 * time.Millisecond
	clockSyncEvery  = 500 * time.Millisecond
	stateSaveEvery  = 2 * time.Second
	finalWaitEvery  = 10 * time.Second
	rebuildPause    = 500 * time.Millisecond
	maxRebuildPause = 5 * time.Second
	healthyRun      = 20 * time.Second
	// A read or a batch must answer fast: a quote has 4.5 real seconds in the daytime.
	liveReadTimeout = 3 * time.Second
	liveActTimeout  = 5 * time.Second
	// maxBlind is how long reads may fail in a row before the books are rebuilt. A single
	// failed read is just retried at the next tick: the engine's books are still right.
	maxBlind = 12 * time.Second
	// maxExtrapolateMin bounds how far the clock estimate runs ahead of its last reading.
	maxExtrapolateMin = 30
	maxSendRounds     = 6
	defaultLiveLevel  = 8
	stateFile         = "engine-state.json"
)

// LiveOptions describe a run on a world whose clock moves by itself.
type LiveOptions struct {
	Root   string
	Params engine.Params
	Label  string
	Out    io.Writer     // progress lines; nil is quiet
	Trace  io.Writer     // engine decisions; nil is quiet
	Poll   time.Duration // how often to read new events; 0 means the default
}

// clockFunc reads the world's current minute from the server.
type clockFunc func(ctx context.Context) (int, error)

// Final waits for the team's final (or test final), plays it from the live clock, and
// survives server restarts and its own errors by rebuilding its books from the world.
func Final(ctx context.Context, c *api.Client, m *geo.Map, opt LiveOptions) error {
	pause := rebuildPause
	for {
		world, err := waitForFinal(ctx, c, opt.Out)
		if err != nil {
			return err
		}
		clock := func(ctx context.Context) (int, error) {
			f, err := c.Final(ctx)
			if err != nil {
				return 0, err
			}
			return f.Now, nil
		}
		joined := time.Now()
		err = PlayLive(ctx, c, world, m, clock, opt)
		switch {
		case err == nil:
			return nil
		case ctx.Err() != nil:
			return ctx.Err()
		}
		// The world keeps going without us: say what happened and get back in. A run
		// that failed right away waits longer each time, so a fault that repeats does
		// not spin; one that played for a while starts again at once.
		if time.Since(joined) > healthyRun {
			pause = rebuildPause
		}
		say(opt.Out, "live: %v — rebuilding from the world in %s", err, pause)
		if err := sleep(ctx, pause); err != nil {
			return err
		}
		pause = min(2*pause, maxRebuildPause)
	}
}

func say(out io.Writer, format string, args ...any) {
	if out != nil {
		fmt.Fprintf(out, time.Now().Format("15:04:05 ")+format+"\n", args...)
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// waitForFinal returns the team's final world once one is set and has not ended.
func waitForFinal(ctx context.Context, c *api.Client, out io.Writer) (*api.World, error) {
	for said := false; ; said = true {
		f, err := c.Final(ctx)
		switch {
		case err == nil:
			world := c.World(f.WorldID)
			snap, serr := world.Snapshot(ctx)
			if serr == nil && snap.Now < snap.EndsAt {
				say(out, "final world %s, starts %s, world minute %d", f.WorldID, f.StartsAt, f.Now)
				return world, nil
			}
		case api.StatusOf(err) == 404 || api.IsUnreachable(err):
			// Not set yet, or the server is restarting: keep waiting.
		default:
			return nil, fmt.Errorf("look up final: %w", err)
		}
		if !said {
			say(out, "no final is set (or the last one has ended); waiting for it")
		}
		if err := sleep(ctx, finalWaitEvery); err != nil {
			return nil, err
		}
	}
}

// worldClock estimates the world's minute between reads from the published speeds.
type worldClock struct {
	world     int
	at        time.Time
	lastExact int  // the previous exact reading
	still     bool // two exact readings in a row gave the same minute: the clock is not moving
}

// secondsPerHour is the real time one world hour takes at world minute t (API.md, the final).
func secondsPerHour(t int) float64 {
	day, minute := cal.DayOf(t), cal.MinuteOfDay(t)
	daytime := minute >= 6*cal.Hour && minute < 22*cal.Hour
	switch {
	case !daytime || day == 6 || day > 7:
		return 3
	case day == 5:
		return 6
	case day == 7:
		return 9
	}
	return 18
}

// now walks the estimate forward minute by minute, so a change of speed (06:00, 22:00)
// inside the interval is followed. It does not move while the world's clock stands
// still (before the start, or while the server restarts).
func (c *worldClock) now() int {
	if c.still {
		return c.world
	}
	left := time.Since(c.at).Seconds()
	t := c.world
	for i := 0; i < maxExtrapolateMin; i++ {
		perMinute := secondsPerHour(t) / float64(cal.Hour)
		if left < perMinute {
			break
		}
		left -= perMinute
		t++
	}
	return t
}

// sync takes a fresh reading. An event's time only ever moves the clock forward; a
// reading from the server (exact) may also pull an estimate back.
func (c *worldClock) sync(world int, exact bool) {
	if exact {
		c.still = world == c.lastExact
		c.lastExact = world
		c.world, c.at = world, time.Now()
		return
	}
	if world > c.now() {
		c.world, c.at, c.still = world, time.Now(), false
	}
}

// PlayLive plays one world from its live clock until it ends. It returns an error when
// the world cannot be reached for a while, an action's fate is unknown, or the engine
// itself fails; the caller starts it again, and it rebuilds its books from the world.
func PlayLive(ctx context.Context, c *api.Client, world *api.World, m *geo.Map, clock clockFunc, opt LiveOptions) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("engine panic: %v\n%s", r, debug.Stack())
		}
	}()
	tape := &api.Tape{}
	ctx = context.WithValue(api.WithRecorder(ctx, tape), tapeKey{}, tape)
	// History, then the snapshot, then the events of the gap between the two. What
	// happened before the snapshot is already in it; only later events are acted on.
	history, err := world.Events(ctx, -1, 0)
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}
	snap, err := world.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	if snap.Now >= snap.EndsAt {
		return nil
	}
	seq := lastSeq(history, -1)
	gap, err := world.Events(ctx, seq, 0)
	if err != nil {
		return fmt.Errorf("read events after the snapshot: %w", err)
	}
	var fresh []api.Event
	for i := range gap {
		if gap[i].At <= snap.Now {
			history = append(history, gap[i])
		} else {
			fresh = append(fresh, gap[i])
		}
	}
	seq = lastSeq(gap, seq)
	roster, err := world.Roster(ctx)
	if err != nil {
		return fmt.Errorf("roster: %w", err)
	}
	level, kind := worldInfo(ctx, c, world.ID)
	started := time.Now()
	meta := store.Meta{
		RunID:     fmt.Sprintf("%s-live-L%d-%s", started.Format("20060102-150405"), level, shortID(world.ID)),
		StartedAt: started, WorldID: world.ID, Kind: kind, Level: level, Policy: opt.Label, Params: opt.Params.AsMap(),
		Engine: engine.Version, EngineHash: engine.SourceHash, EngineNote: engine.VersionNote,
	}
	run, err := store.NewRun(opt.Root, meta)
	if err != nil {
		return err
	}
	tape.Attach(run.HTTP)
	summary := store.Summary{Meta: meta}
	fail := func(cause error) error {
		summary.Error = cause.Error()
		summary.Requests = tape.Count()
		summary.EndedAt = time.Now()
		summary.Seconds = summary.EndedAt.Sub(started).Seconds()
		return errors.Join(cause, run.Finish(summary, nil))
	}
	if err := run.SaveRoster(roster); err != nil {
		return fail(err)
	}
	eng := engine.New(level, opt.Params, m)
	if opt.Trace != nil {
		eng.Logf = func(format string, args ...any) { fmt.Fprintf(opt.Trace, format+"\n", args...) }
	}
	statePath := filepath.Join(opt.Root, "live-"+world.ID+"-"+stateFile)
	eng.Rejoin(snap, roster, history)
	if err := eng.LoadState(statePath); err != nil {
		say(opt.Out, "live: %v (continuing with the replayed market)", err)
	}
	eng.Resume(snap)
	for i := range history {
		run.Event(history[i].Raw)
	}
	say(opt.Out, "joined world %s at %s with %d past events; %s", world.ID, cal.Format(snap.Now), len(history), ScoreLine(&snap.Score))
	l := &liveLoop{world: world, eng: eng, run: run, clock: clock, opt: opt, seq: seq, endsAt: snap.EndsAt, statePath: statePath, pending: fresh}
	l.wc.sync(snap.Now, true)
	playErr := l.loop(ctx)
	summary.Stats = eng.Stats()
	if playErr != nil {
		if score, serr := world.Score(context.WithoutCancel(ctx)); serr == nil {
			summary.Score = *score
		}
		return fail(playErr)
	}
	summary.Requests = tape.Count()
	final, err := finish(ctx, world, eng, run, summary, false, nil)
	say(opt.Out, "END %s", ScoreLine(&final.Score))
	return err
}

func lastSeq(events []api.Event, seq int) int {
	for i := range events {
		seq = max(seq, events[i].Seq)
	}
	return seq
}

// shortID is the tail of a world id, for run names.
func shortID(id string) string {
	const tail = 6
	if len(id) <= tail {
		return id
	}
	return id[len(id)-tail:]
}

// worldInfo looks up the level and kind of a world; the final is level 8.
func worldInfo(ctx context.Context, c *api.Client, id string) (int, string) {
	worlds, err := c.Worlds(ctx)
	if err == nil {
		for _, w := range worlds {
			if w.WorldID == id {
				return w.Level, w.Kind
			}
		}
	}
	return defaultLiveLevel, kindFinal
}

type liveLoop struct {
	world     *api.World
	eng       *engine.Engine
	run       *store.Run
	clock     clockFunc
	opt       LiveOptions
	wc        worldClock
	seq       int
	endsAt    int
	statePath string
	pending   []api.Event // events read with the snapshot, to act on in the first round
}

func (l *liveLoop) loop(ctx context.Context) error {
	poll := l.opt.Poll
	if poll <= 0 {
		poll = defaultPoll
	}
	// What Resume decided goes out first, then the events that came with the snapshot.
	if err := l.round(ctx, l.pending); err != nil {
		return err
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	var lastSync, lastSave time.Time
	lastRead := time.Now()
	day := cal.DayOf(l.eng.Now())
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if time.Since(lastSync) >= clockSyncEvery {
			if now, err := l.readClock(ctx); err == nil {
				l.wc.sync(now, true)
				lastSync = time.Now()
			}
		}
		events, err := l.readEvents(ctx)
		switch {
		case err == nil:
			lastRead = time.Now()
		case ctx.Err() != nil:
			return ctx.Err()
		case time.Since(lastRead) > maxBlind:
			return fmt.Errorf("no events for %s: %w", maxBlind, err)
		}
		// On a failed read the round still runs: the engine's timers go on by the clock.
		if err := l.round(ctx, events); err != nil {
			return err
		}
		if time.Since(lastSave) >= stateSaveEvery {
			lastSave = time.Now()
			if err := l.eng.SaveState(l.statePath); err != nil {
				say(l.opt.Out, "live: %v", err)
			}
			_ = l.run.Flush()
		}
		if d := cal.DayOf(l.eng.Now()); d > day {
			l.report(ctx, day)
			day = d
		}
		if l.eng.Now() >= l.endsAt && l.ended(ctx) {
			return nil
		}
	}
}

func (l *liveLoop) readClock(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, liveReadTimeout)
	defer cancel()
	return l.clock(ctx)
}

func (l *liveLoop) readEvents(ctx context.Context) ([]api.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, liveReadTimeout)
	defer cancel()
	events, err := l.world.Events(ctx, l.seq, 0)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// ended confirms with the server that the world is over; the clock estimate alone is not enough.
func (l *liveLoop) ended(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, liveReadTimeout)
	defer cancel()
	snap, err := l.world.Snapshot(ctx)
	if err != nil {
		return false
	}
	if snap.Now >= snap.EndsAt {
		return true
	}
	l.wc.sync(snap.Now, true)
	return false
}

// round feeds new events to the engine at the current clock and sends what it decides.
func (l *liveLoop) round(ctx context.Context, events []api.Event) error {
	for i := range events {
		l.run.Event(events[i].Raw)
		l.seq = max(l.seq, events[i].Seq)
		l.wc.sync(events[i].At, false)
	}
	l.eng.SetNow(l.wc.now())
	l.eng.Handle(events)
	return l.sendAll(ctx)
}

// sendAll sends what the engine decided. Quotes go first and alone: a quote has 15 world
// minutes, a few real seconds, and must not wait for the tokens of a batch of posts.
// Results can lead to more actions, so it goes a few rounds.
func (l *liveLoop) sendAll(ctx context.Context) error {
	for round := 0; round < maxSendRounds; round++ {
		sent := l.eng.Drain()
		if len(sent) == 0 {
			return nil
		}
		var quotes, rest []engine.Pending
		for _, p := range sent {
			if p.Action.Type == api.ActQuote {
				quotes = append(quotes, p)
			} else {
				rest = append(rest, p)
			}
		}
		for _, group := range [][]engine.Pending{quotes, rest} {
			for i := 0; i < len(group); i += api.MaxActionsPerCall {
				if err := l.act(ctx, group[i:min(i+api.MaxActionsPerCall, len(group))]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// act sends one batch. A batch the server refuses as a whole (4xx: nothing was applied)
// is sent again action by action, and an action refused alone is reported to the engine
// as refused. Only when the fate of the batch is unknown (no answer) does it return an
// error, and the books are rebuilt from the world.
func (l *liveLoop) act(ctx context.Context, batch []engine.Pending) error {
	if len(batch) == 0 {
		return nil
	}
	actCtx, cancel := context.WithTimeout(ctx, liveActTimeout)
	results, err := l.world.Act(actCtx, actionsOf(batch))
	cancel()
	status := api.StatusOf(err)
	switch {
	case err == nil:
		logActions(l.run, l.eng.Now(), batch, results)
		l.eng.Resolve(batch, results)
		return nil
	case status >= 400 && status < 500 && len(batch) > 1:
		for i := range batch {
			if err := l.act(ctx, batch[i:i+1]); err != nil {
				return err
			}
		}
		return nil
	case status >= 400 && status < 500:
		refused := []api.ActionResult{{OK: false, Reason: err.Error()}}
		logActions(l.run, l.eng.Now(), batch, refused)
		l.eng.Resolve(batch, refused)
		return nil
	}
	return fmt.Errorf("send %d actions: %w", len(batch), err)
}

func (l *liveLoop) report(ctx context.Context, day int) {
	ctx, cancel := context.WithTimeout(ctx, liveReadTimeout)
	defer cancel()
	score, err := l.world.Score(ctx)
	if err != nil {
		return
	}
	say(l.opt.Out, "%-9s %s", weekdays[day%7], ScoreLine(score))
}

// SimulatedClock moves a practice world's clock like a live one, for rehearsals: every
// tick it advances the world by step minutes. The returned function reads the minute reached.
func SimulatedClock(ctx context.Context, world *api.World, step int, tick time.Duration) clockFunc {
	var now atomic.Int64
	go func() {
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		at := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if at == 0 {
				snap, err := world.Snapshot(ctx)
				if err != nil {
					continue
				}
				at = snap.Now
			}
			adv, err := world.AdvanceUntil(ctx, nil, at+step, nil)
			if err != nil {
				continue
			}
			at = adv.Now
			now.Store(int64(at))
		}
	}()
	return func(context.Context) (int, error) { return int(now.Load()), nil }
}
