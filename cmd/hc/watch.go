package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
	"progresful-transfers/internal/config"
)

const (
	feedDir        = "final-feed"
	feedRetry      = 3 * time.Second
	feedPrintEvery = 15 * time.Second
	feedTopTeams   = 5
	statusRunning  = "running"
	statusEnded    = "ended"
)

// feedLine is one stored message of the live feed with the wall clock it arrived at.
type feedLine struct {
	At   time.Time       `json:"t"`
	Feed json.RawMessage `json:"feed"`
}

// cmdWatch records the public live feed of a final or test final: every team's margin,
// revenue, orders and on-time share about every two seconds. It needs no team key. The
// record shows how the leaders play the same week: their price per order, how fast
// revenue follows orders, and when they win.
func cmdWatch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	team := fs.String("team", "", "a team to always show in the progress lines (id as in the rankings)")
	root := fs.String("root", defaultRoot, "folder of run logs")
	stop := fs.Bool("until-end", true, "stop when a final that was running has ended")
	if err := fs.Parse(args); err != nil {
		return err
	}
	url := os.Getenv("SPREAD_URL")
	if url == "" {
		url = config.DefaultURL
	}
	dir := filepath.Join(*root, feedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, "feed-"+time.Now().Format("20060102-150405")+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Printf("recording the public live feed to %s (Ctrl-C stops)\n", path)

	w := &watcher{out: out, team: *team}
	for {
		err := api.StreamLeaderboard(ctx, url, w.handle)
		switch {
		case ctx.Err() != nil:
			return nil
		case errors.Is(err, errFeedEnded) && *stop:
			fmt.Printf("the final has ended: %d messages recorded in %s\n", w.count, path)
			return nil
		case err != nil && !errors.Is(err, errFeedEnded):
			fmt.Printf("%s feed: %v; connecting again\n", time.Now().Format("15:04:05"), err)
		}
		_ = out.Flush()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(feedRetry):
		}
	}
}

var errFeedEnded = errors.New("final ended")

type watcher struct {
	out       *bufio.Writer
	team      string
	count     int
	wasLive   bool
	lastPrint time.Time
	lastState string
}

func (w *watcher) handle(raw []byte) error {
	var feed api.FinalFeed
	if err := json.Unmarshal(raw, &feed); err != nil {
		return nil // a message that does not parse is skipped, the stream goes on
	}
	// Idle messages ("none") are not stored: there can be hours of them.
	if feed.Status != "none" || feed.Status != w.lastState {
		line, err := json.Marshal(feedLine{At: time.Now(), Feed: raw})
		if err == nil {
			w.out.Write(line)
			w.out.WriteByte('\n')
			w.count++
		}
	}
	if feed.Status != w.lastState || time.Since(w.lastPrint) >= feedPrintEvery {
		w.print(&feed)
		w.lastPrint, w.lastState = time.Now(), feed.Status
		_ = w.out.Flush()
	}
	if feed.Status == statusRunning {
		w.wasLive = true
	}
	if feed.Status == statusEnded && w.wasLive {
		return errFeedEnded
	}
	return nil
}

func (w *watcher) print(feed *api.FinalFeed) {
	clock := ""
	if feed.Status == statusRunning || feed.Status == statusEnded {
		clock = " world " + cal.Format(feed.Now)
	}
	fmt.Printf("%s feed %s%s, %d teams", time.Now().Format("15:04:05"), feed.Status, clock, len(feed.Teams))
	teams := append([]api.FeedTeam(nil), feed.Teams...)
	sort.SliceStable(teams, func(i, j int) bool {
		if teams[i].Ranked != teams[j].Ranked {
			return teams[i].Ranked
		}
		return teams[i].MarginCents > teams[j].MarginCents
	})
	for i, t := range teams {
		if i < feedTopTeams || t.Team == w.team {
			fmt.Printf(" | %d %s €%.0f %d ord %.0f%%", i+1, clip(t.Team, 14), float64(t.MarginCents)/100, t.WonOrders, 100*t.OnTimeShare)
		}
	}
	fmt.Println()
}
