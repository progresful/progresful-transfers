// Package store keeps a durable log of every run: events, actions, summary and reveal,
// plus one index line per run so progress can be tracked from the CLI.
package store

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"highwaycoins/internal/api"
)

const (
	indexFile   = "index.jsonl"
	metaFile    = "meta.json"
	eventsFile  = "events.jsonl"
	actionsFile = "actions.jsonl"
	// decisionsFile holds how each price request was answered and why (for the web view).
	decisionsFile = "decisions.jsonl"
	summaryFile   = "summary.json"
	revealFile    = "reveal.json"
	rosterFile    = "roster.json"
	tapeFile      = "http.jsonl.gz"
	dirPerm       = 0o755
	filePerm      = 0o644
)

var indexMu sync.Mutex

// Meta identifies a run.
type Meta struct {
	RunID     string    `json:"runId"`
	StartedAt time.Time `json:"startedAt"`
	WorldID   string    `json:"worldId"`
	Kind      string    `json:"kind"` // practice | board | final
	Level     int       `json:"level"`
	Seed      string    `json:"seed"`
	Policy    string    `json:"policy"`
	// Engine, EngineHash and EngineNote say which decision logic played the run: the
	// engine's version, the hash of its source, and what that version does.
	Engine     string         `json:"engine,omitempty"`
	EngineHash string         `json:"engineHash,omitempty"`
	EngineNote string         `json:"engineNote,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	Note       string         `json:"note,omitempty"`
}

// Summary is the index line of a finished run.
type Summary struct {
	Meta
	EndedAt    time.Time      `json:"endedAt"`
	Seconds    float64        `json:"seconds"`
	Requests   int            `json:"httpRequests"`
	Score      api.Score      `json:"score"`
	Stats      map[string]any `json:"stats,omitempty"`
	Error      string         `json:"error,omitempty"`
	WorldEnded bool           `json:"worldEnded"`
}

// Run is the open log of one run.
type Run struct {
	Meta    Meta
	Dir     string
	root    string
	mu      sync.Mutex
	events  *bufio.Writer
	actions *bufio.Writer
	notes   *bufio.Writer // decisions.jsonl
	tape    *gzip.Writer
	tapeBuf *bufio.Writer
	files   []*os.File
}

// NewRun creates runs/<runId>/ and writes meta.json.
func NewRun(root string, meta Meta) (*Run, error) {
	dir := filepath.Join(root, meta.RunID)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}
	r := &Run{Meta: meta, Dir: dir, root: root}
	if err := writeJSON(filepath.Join(dir, metaFile), meta); err != nil {
		return nil, err
	}
	var err error
	if r.events, err = r.open(eventsFile); err != nil {
		return nil, err
	}
	if r.actions, err = r.open(actionsFile); err != nil {
		r.closeFiles()
		return nil, err
	}
	if r.notes, err = r.open(decisionsFile); err != nil {
		r.closeFiles()
		return nil, err
	}
	tape, err := r.open(tapeFile)
	if err != nil {
		r.closeFiles()
		return nil, err
	}
	r.tape = gzip.NewWriter(tape)
	r.tapeBuf = tape
	return r, nil
}

func (r *Run) closeFiles() {
	for _, f := range r.files {
		_ = f.Close()
	}
}

// HTTP appends one request/response exchange to the run's tape (gzip JSON lines): the
// full record of what the run sent and received, with response times.
func (r *Run) HTTP(ex api.Exchange) {
	line, err := json.Marshal(ex)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tape == nil {
		return
	}
	r.tape.Write(line)
	r.tape.Write([]byte{'\n'})
}

// ReadTape loads the exchanges of a run, in order.
func ReadTape(dir string) ([]api.Exchange, error) {
	f, err := os.Open(filepath.Join(dir, tapeFile))
	if err != nil {
		return nil, fmt.Errorf("open tape: %w", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open tape: %w", err)
	}
	defer zr.Close()
	var out []api.Exchange
	dec := json.NewDecoder(zr)
	for dec.More() {
		var ex api.Exchange
		if err := dec.Decode(&ex); err != nil {
			return out, fmt.Errorf("read tape after %d exchanges: %w", len(out), err)
		}
		out = append(out, ex)
	}
	return out, nil
}

// ReadMeta loads the identity of a run.
func ReadMeta(dir string) (Meta, error) {
	var meta Meta
	data, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return meta, fmt.Errorf("read meta: %w", err)
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("decode meta: %w", err)
	}
	return meta, nil
}

func (r *Run) open(name string) (*bufio.Writer, error) {
	f, err := os.OpenFile(filepath.Join(r.Dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	r.files = append(r.files, f)
	return bufio.NewWriterSize(f, 1<<16), nil
}

// Event appends one raw event.
func (r *Run) Event(raw json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events.Write(raw)
	r.events.WriteByte('\n')
}

// Decision appends one record of how a price request was answered (any JSON value).
func (r *Run) Decision(v any) {
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notes.Write(line)
	r.notes.WriteByte('\n')
}

// ActionRecord is one sent action with its result.
type ActionRecord struct {
	At     int              `json:"at"`
	Action api.Action       `json:"action"`
	Result api.ActionResult `json:"result"`
}

// Action appends one action and its result.
func (r *Run) Action(at int, a api.Action, res api.ActionResult) {
	line, err := json.Marshal(ActionRecord{At: at, Action: a, Result: res})
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions.Write(line)
	r.actions.WriteByte('\n')
}

// SaveRoster stores the driver list of the world.
func (r *Run) SaveRoster(roster []api.DriverProfile) error {
	return writeJSON(filepath.Join(r.Dir, rosterFile), roster)
}

// Lines writes n JSON lines to a file of the run (per-order reports and the like).
func (r *Run) Lines(name string, n int, line func(i int) any) error {
	f, err := os.OpenFile(filepath.Join(r.Dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for i := 0; i < n; i++ {
		data, err := json.Marshal(line(i))
		if err != nil {
			return fmt.Errorf("encode %s line %d: %w", name, i, err)
		}
		w.Write(data)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// Flush pushes buffered lines to disk.
func (r *Run) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	errs := []error{r.events.Flush(), r.actions.Flush(), r.notes.Flush()}
	if r.tape != nil {
		// A sync flush keeps the tape readable up to here if the process dies.
		errs = append(errs, r.tape.Flush(), r.tapeBuf.Flush())
	}
	return errors.Join(errs...)
}

// Abandon closes the log of a run that could not start, without an index line.
func (r *Run) Abandon() {
	_ = r.Flush()
	_ = r.closeTape()
	for _, f := range r.files {
		_ = f.Close()
	}
}

// closeTape ends the gzip stream of the tape.
func (r *Run) closeTape() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tape == nil {
		return nil
	}
	err := errors.Join(r.tape.Close(), r.tapeBuf.Flush())
	r.tape = nil
	return err
}

// Finish writes the summary (and reveal, when given), appends the index line and closes the log.
func (r *Run) Finish(s Summary, reveal json.RawMessage) error {
	errs := []error{r.Flush(), r.closeTape()}
	for _, f := range r.files {
		errs = append(errs, f.Close())
	}
	if len(reveal) > 0 {
		errs = append(errs, os.WriteFile(filepath.Join(r.Dir, revealFile), reveal, filePerm))
	}
	errs = append(errs, writeJSON(filepath.Join(r.Dir, summaryFile), s), appendIndex(r.root, s))
	return errors.Join(errs...)
}

func appendIndex(root string, s Summary) error {
	line, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode summary: %w", err)
	}
	indexMu.Lock()
	defer indexMu.Unlock()
	f, err := os.OpenFile(filepath.Join(root, indexFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	return nil
}

// Index reads every finished run, oldest first.
func Index(root string) ([]Summary, error) {
	f, err := os.Open(filepath.Join(root, indexFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer f.Close()
	var out []Summary
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<26)
	for scanner.Scan() {
		var s Summary
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue // a torn line from an interrupted run
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out, scanner.Err()
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
