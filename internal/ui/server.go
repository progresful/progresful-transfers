// Package ui serves the dispatcher's web view: a live map and replay of a run, read from
// the run logs under runs/. It is a read-only side process: it never calls the Spread API
// with the team key and never touches the engine, so it cannot cost the engine a request
// or a decision. The only outside connection it makes is to the public feed of a final.
package ui

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"highwaycoins/internal/api"
)

//go:embed static
var static embed.FS

const (
	eventsFile   = "events.jsonl"
	actionsFile  = "actions.jsonl"
	metaFile     = "meta.json"
	summaryFile  = "summary.json"
	rosterFile   = "roster.json"
	mapFile      = "map.json"
	tailEvery    = 400 * time.Millisecond
	keepAlive    = 15 * time.Second
	liveWithin   = 90 * time.Second // a run that wrote this lately is being played
	maxRuns      = 200
	defaultRuns  = 40
	maxLineBytes = 4 << 20
	// viewedKind is four fifths of a run's events and nothing the map shows.
	viewedKind = `"kind":"gig_viewed"`
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,120}$`)

// Server serves the web view of the runs under Root.
type Server struct {
	Root    string // the runs folder
	FeedURL string // base URL of the Spread server, for the public feed of a final
	Team    string // the team to highlight in the feed (its name or id), may be empty
	// StaticDir serves the page from a folder instead of the copy compiled into the
	// binary: for working on the page without rebuilding.
	StaticDir string
}

// Handler routes the web view's requests.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	files, err := fs.Sub(static, "static")
	if err != nil {
		panic(err) // the folder is compiled in
	}
	if s.StaticDir != "" {
		files = os.DirFS(s.StaticDir)
	}
	mux.Handle("GET /", http.FileServer(http.FS(files)))
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/map", s.worldMap)
	mux.HandleFunc("GET /api/runs", s.runs)
	mux.HandleFunc("GET /api/runs/{id}/roster", s.roster)
	mux.HandleFunc("GET /api/runs/{id}/stream", s.stream)
	mux.HandleFunc("GET /api/feed", s.feed)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode answer", http.StatusInternalServerError)
	}
}

func (s *Server) config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"team": s.Team})
}

// worldMap serves the places of the map as the engine last saved it.
func (s *Server) worldMap(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.Root, mapFile))
}

// runDir returns the folder of a run, or "" for an id that is not a run of this root.
func (s *Server) runDir(id string) string {
	if !runIDPattern.MatchString(id) {
		return ""
	}
	dir := filepath.Join(s.Root, id)
	if info, err := os.Stat(filepath.Join(dir, metaFile)); err != nil || info.IsDir() {
		return ""
	}
	return dir
}

// runInfo is one row of the run list.
type runInfo struct {
	ID        string          `json:"id"`
	Live      bool            `json:"live"`     // still being written
	Complete  bool            `json:"complete"` // played to the end of its world
	StartedAt time.Time       `json:"startedAt"`
	Meta      json.RawMessage `json:"meta"`
	Summary   json.RawMessage `json:"summary,omitempty"`
}

// runs lists the newest runs, a run that is still being played first.
func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	limit := defaultRuns
	if n, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && n > 0 {
		limit = min(n, maxRuns)
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		http.Error(w, "read runs", http.StatusInternalServerError)
		return
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && runIDPattern.MatchString(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids))) // run ids start with their start time
	out := make([]runInfo, 0, limit)
	for _, id := range ids {
		if len(out) >= limit {
			break
		}
		dir := filepath.Join(s.Root, id)
		meta, err := os.ReadFile(filepath.Join(dir, metaFile))
		if err != nil || !json.Valid(meta) {
			continue
		}
		info := runInfo{ID: id, Meta: meta}
		var m struct {
			StartedAt time.Time `json:"startedAt"`
		}
		_ = json.Unmarshal(meta, &m)
		info.StartedAt = m.StartedAt
		if summary, err := os.ReadFile(filepath.Join(dir, summaryFile)); err == nil && json.Valid(summary) {
			info.Summary = summary
			var sum struct {
				WorldEnded bool `json:"worldEnded"`
			}
			_ = json.Unmarshal(summary, &sum)
			info.Complete = sum.WorldEnded
		} else if written, err := os.Stat(filepath.Join(dir, eventsFile)); err == nil {
			// No summary yet: the run is being played if it wrote lately, else it was stopped.
			info.Live = time.Since(written.ModTime()) < liveWithin
		}
		out = append(out, info)
	}
	writeJSON(w, out)
}

// roster serves where each driver of the run's world lives.
func (s *Server) roster(w http.ResponseWriter, r *http.Request) {
	dir := s.runDir(r.PathValue("id"))
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(dir, rosterFile))
}

// sse prepares a response for server-sent events and returns its flusher.
func sse(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no") // a reverse proxy must not hold the stream back
	return flusher, true
}

// tail follows one JSON-lines file of a run: it hands over every complete line once.
type tail struct {
	path string
	file *os.File
	rd   *bufio.Reader
	part []byte // a line whose end is not on disk yet
}

// lines reads what is new in the file since the last call.
func (t *tail) lines(emit func(line []byte) error) error {
	if t.file == nil {
		f, err := os.Open(t.path)
		if errors.Is(err, os.ErrNotExist) {
			return nil // the run has not written it yet
		}
		if err != nil {
			return fmt.Errorf("open %s: %w", filepath.Base(t.path), err)
		}
		t.file, t.rd = f, bufio.NewReaderSize(f, 1<<16)
	}
	for {
		chunk, err := t.rd.ReadBytes('\n')
		t.part = append(t.part, chunk...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read %s: %w", filepath.Base(t.path), err)
		}
		line := t.part[:len(t.part)-1]
		t.part = t.part[:0]
		if len(line) == 0 || len(line) > maxLineBytes {
			continue
		}
		if err := emit(line); err != nil {
			return err
		}
	}
}

func (t *tail) close() {
	if t.file != nil {
		_ = t.file.Close()
	}
}

// stream sends a run's events and actions as server-sent events: everything on disk
// first, then what the run writes while it is played, until the run has ended. Each
// message is {"e": <event>}, {"a": <action>} or, for a driver's look at a gig (four
// fifths of all events), the short {"v": [<minute>, <gig id>]}.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	dir := s.runDir(r.PathValue("id"))
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	flusher, ok := sse(w)
	if !ok {
		return
	}
	events, actions := &tail{path: filepath.Join(dir, eventsFile)}, &tail{path: filepath.Join(dir, actionsFile)}
	defer events.close()
	defer actions.close()
	send := func(key string) func([]byte) error {
		return func(line []byte) error {
			if key == "e" && containsViewed(line) {
				var view struct {
					At    int    `json:"at"`
					GigID string `json:"gigId"`
				}
				if json.Unmarshal(line, &view) != nil || view.GigID == "" {
					return nil
				}
				_, err := fmt.Fprintf(w, "data: {\"v\":[%d,%q]}\n\n", view.At, view.GigID)
				return err
			}
			_, err := fmt.Fprintf(w, "data: {%q:%s}\n\n", key, line)
			return err
		}
	}
	lastBeat := time.Now()
	for {
		// Actions first: a gig's post names its cars before the events about the gig.
		if err := actions.lines(send("a")); err != nil {
			return
		}
		if err := events.lines(send("e")); err != nil {
			return
		}
		_, statErr := os.Stat(filepath.Join(dir, summaryFile))
		ended := statErr == nil
		if ended {
			// The summary is written after the last lines: read once more, then stop.
			_ = actions.lines(send("a"))
			_ = events.lines(send("e"))
			fmt.Fprint(w, "event: end\ndata: {}\n\n")
			flusher.Flush()
			return
		}
		if time.Since(lastBeat) > keepAlive {
			fmt.Fprint(w, ": keep-alive\n\n")
			lastBeat = time.Now()
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-time.After(tailEvery):
		}
	}
}

func containsViewed(line []byte) bool {
	return bytes.Contains(line, []byte(viewedKind))
}

// feed relays the public feed of a final (every team's score every two seconds). It
// needs no key and is not counted against the team's requests.
func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	flusher, ok := sse(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	err := api.StreamLeaderboard(ctx, s.FeedURL, func(raw []byte) error {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(w, "event: feed-error\ndata: %q\n\n", err.Error())
		flusher.Flush()
	}
}
