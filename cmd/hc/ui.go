package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"progresful-transfers/internal/config"
	"progresful-transfers/internal/ui"
)

const (
	uiDefaultAddr   = "127.0.0.1:8790"
	uiHeaderTimeout = 10 * time.Second
	uiShutdownWait  = 3 * time.Second
)

// cmdUI serves the web view of the run logs: a live map of the run that is being played
// and a replay of any logged run. It only reads the run folders, and for the ranking of
// a final it reads the public feed; it never uses the team key.
func cmdUI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	addr := fs.String("addr", uiDefaultAddr, "address to listen on")
	root := fs.String("root", defaultRoot, "folder of run logs")
	team := fs.String("team", "", "team to highlight in the ranking of a final (name or id as in the rankings)")
	staticDir := fs.String("static", "", "serve the page from this folder instead of the built-in copy (for working on it)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	url := os.Getenv("SPREAD_URL")
	if url == "" {
		url = config.DefaultURL
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           (&ui.Server{Root: *root, FeedURL: url, Team: *team, StaticDir: *staticDir}).Handler(),
		ReadHeaderTimeout: uiHeaderTimeout,
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	fmt.Printf("web view on http://%s (Ctrl-C stops)\n", *addr)
	select {
	case err := <-done:
		return fmt.Errorf("serve the web view: %w", err)
	case <-ctx.Done():
	}
	stop, cancel := context.WithTimeout(context.Background(), uiShutdownWait)
	defer cancel()
	if err := srv.Shutdown(stop); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("stop the web view: %w", err)
	}
	return nil
}
