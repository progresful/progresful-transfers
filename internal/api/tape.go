package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Exchange is one HTTP request to the server with its answer and timing: the full
// record of what a run sent and received.
type Exchange struct {
	N        int             `json:"n"`                  // order of the request within the run
	At       time.Time       `json:"at"`                 // wall clock when the request was sent
	Method   string          `json:"method"`             //
	Path     string          `json:"path"`               // path and query, without the server address
	Request  json.RawMessage `json:"request,omitempty"`  // body sent
	Status   int             `json:"status"`             // HTTP status; 0 when the server did not answer
	Response json.RawMessage `json:"response,omitempty"` // body received (JSON as is, other text as a JSON string)
	Ms       float64         `json:"ms"`                 // response time: request sent to body read
	WaitMs   float64         `json:"waitMs,omitempty"`   // time waited for a rate-limit token before sending
	Limit    string          `json:"limit,omitempty"`    // RateLimit-Policy of the answer
	Left     string          `json:"left,omitempty"`     // RateLimit-Remaining of the answer
	Error    string          `json:"error,omitempty"`    // transport error, when there was no answer
}

// Recorder receives every exchange of the requests made with its context.
type Recorder interface {
	Record(Exchange)
}

type recorderKey struct{}

// WithRecorder returns a context whose requests are reported to rec.
func WithRecorder(ctx context.Context, rec Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, rec)
}

func recorderOf(ctx context.Context) Recorder {
	rec, _ := ctx.Value(recorderKey{}).(Recorder)
	return rec
}

// rawJSON keeps a JSON body as is and wraps anything else as a JSON string.
func rawJSON(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	if json.Valid(data) {
		return json.RawMessage(data)
	}
	quoted, err := json.Marshal(string(data))
	if err != nil {
		return nil
	}
	return quoted
}

// Tape buffers exchanges until a sink is attached, then passes them on in order. A run
// starts recording before its log exists (the request that creates the world), so the
// first exchanges wait here.
type Tape struct {
	mu      sync.Mutex
	n       int
	pending []Exchange
	sink    func(Exchange)
}

// Record numbers an exchange and hands it to the sink, or keeps it until there is one.
func (t *Tape) Record(ex Exchange) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n++
	ex.N = t.n
	if t.sink == nil {
		t.pending = append(t.pending, ex)
		return
	}
	t.sink(ex)
}

// Attach sets the sink and flushes what was recorded before.
func (t *Tape) Attach(sink func(Exchange)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sink = sink
	for _, ex := range t.pending {
		sink(ex)
	}
	t.pending = nil
}

// Count is the number of exchanges recorded so far.
func (t *Tape) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n
}

// ReplayTransport stands in for the server: it answers each request with the recorded
// answer of the same position on a tape. It lets a recorded run be played again offline,
// to check that a change to the engine still makes the same decisions (the answers are
// only valid while the requests stay the same; the first different request is an error).
type ReplayTransport struct {
	mu        sync.Mutex
	exchanges []Exchange
	next      int
}

// NewReplayTransport plays the given exchanges in order.
func NewReplayTransport(exchanges []Exchange) *ReplayTransport {
	return &ReplayTransport{exchanges: exchanges}
}

// Remaining is how many recorded exchanges have not been played.
func (r *ReplayTransport) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.exchanges) - r.next
}

// DivergedError says where a replay stopped matching its tape.
type DivergedError struct {
	N      int
	Reason string
}

func (e *DivergedError) Error() string {
	return fmt.Sprintf("replay diverged at request %d: %s", e.N, e.Reason)
}

// RoundTrip implements http.RoundTripper.
func (r *ReplayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, fmt.Errorf("read replayed request: %w", err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.exchanges) {
		return nil, &DivergedError{N: r.next + 1, Reason: fmt.Sprintf("the tape has ended, but the run sends %s %s", req.Method, req.URL.RequestURI())}
	}
	ex := r.exchanges[r.next]
	path := req.URL.RequestURI()
	if ex.Method != req.Method || ex.Path != path {
		return nil, &DivergedError{N: ex.N, Reason: fmt.Sprintf("tape has %s %s, the run sends %s %s", ex.Method, ex.Path, req.Method, path)}
	}
	if !sameJSON(ex.Request, body) {
		return nil, &DivergedError{N: ex.N, Reason: fmt.Sprintf("%s %s: body differs\n  tape: %s\n  run:  %s", ex.Method, ex.Path, clipBytes(ex.Request), clipBytes(body))}
	}
	r.next++
	if ex.Status == 0 {
		return nil, fmt.Errorf("replayed transport error: %s", ex.Error)
	}
	return &http.Response{
		StatusCode: ex.Status,
		Status:     http.StatusText(ex.Status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(ex.Response)),
		Request:    req,
	}, nil
}

func sameJSON(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

const clipLen = 600

func clipBytes(b []byte) string {
	if len(b) > clipLen {
		return string(b[:clipLen]) + "…"
	}
	return string(b)
}
