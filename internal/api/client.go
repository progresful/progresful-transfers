package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// MaxActionsPerCall is the server's batch limit.
	MaxActionsPerCall = 50
	requestTimeout    = 30 * time.Second
	defaultRetryAfter = 100 * time.Millisecond
	maxRateRetries    = 600
	finalBurst        = 50
)

// Error is a request the server refused.
type Error struct {
	Status int
	Msg    string
	Field  string
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("spread: %s (HTTP %d, field %s)", e.Msg, e.Status, e.Field)
	}
	return fmt.Sprintf("spread: %s (HTTP %d)", e.Msg, e.Status)
}

// Unreachable means the server did not answer or is restarting (502-504).
// The request may or may not have been applied: read state before repeating a mutation.
type Unreachable struct{ Msg string }

func (e *Unreachable) Error() string { return "spread unreachable: " + e.Msg }

// IsUnreachable reports whether err is (or wraps) an Unreachable.
func IsUnreachable(err error) bool {
	var u *Unreachable
	return errors.As(err, &u)
}

// StatusOf returns the HTTP status of an API error, or 0.
func StatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

// bucket mirrors one of the server's token buckets from the RateLimit headers.
type bucket struct {
	name      string
	perSecond float64
	tokens    float64
	at        time.Time
}

func newBucket(name string, h http.Header, now time.Time) *bucket {
	perSecond, _ := strconv.ParseFloat(h.Get("RateLimit-Limit"), 64)
	remaining, _ := strconv.ParseFloat(h.Get("RateLimit-Remaining"), 64)
	resetMs, _ := strconv.ParseFloat(h.Get("RateLimit-Reset"), 64)
	tokens := remaining
	if resetMs > 0 && perSecond > 0 {
		// Headers give whole tokens; the wait for the next one gives the fraction.
		tokens += 1 - resetMs/1000*perSecond
	}
	return &bucket{name: name, perSecond: perSecond, tokens: tokens, at: now}
}

// capacity is the most tokens the server's bucket holds: 50 in the final, one second's
// worth in practice (API.md, Limits).
func (b *bucket) capacity() float64 {
	if strings.HasPrefix(b.name, "final-") {
		return finalBurst
	}
	return math.Max(1, b.perSecond)
}

// take reserves the tokens for one request and returns how long to wait first.
func (b *bucket) take(actions int, now time.Time) time.Duration {
	if b.perSecond <= 0 {
		return 0
	}
	cost := 1.0
	if b.name == "final-actions" && actions > 1 {
		cost = float64(actions)
	}
	tokens := math.Min(b.capacity(), b.tokens+now.Sub(b.at).Seconds()*b.perSecond)
	delay := 0.0
	if tokens < cost {
		delay = (cost - tokens) / b.perSecond
	}
	b.tokens = tokens + delay*b.perSecond - cost
	b.at = now.Add(time.Duration(delay * float64(time.Second)))
	return time.Duration(delay * float64(time.Second))
}

// Client talks to one Spread server with one team key. Safe for concurrent use.
type Client struct {
	url  string
	key  string
	http *http.Client

	mu      sync.Mutex
	buckets map[string]*bucket
	routes  map[string]string // "METHOD /path" -> bucket name
}

// NewClient validates the address and key.
func NewClient(url, key string) (*Client, error) {
	url = strings.TrimRight(url, "/")
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("SPREAD_URL %q is not an http(s) address", url)
	}
	if key == "" {
		return nil, errors.New("SPREAD_KEY is not set")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 32
	return &Client{
		url:     url,
		key:     key,
		http:    &http.Client{Transport: transport},
		buckets: map[string]*bucket{},
		routes:  map[string]string{},
	}, nil
}

// SetTransport replaces how requests reach the server (a replay tape instead of the network).
func (c *Client) SetTransport(rt http.RoundTripper) { c.http = &http.Client{Transport: rt} }

// URL returns the server address.
func (c *Client) URL() string { return c.url }

// Key returns the team key (needed by the stream).
func (c *Client) Key() string { return c.key }

func (c *Client) reserve(route string, actions int) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := c.buckets[c.routes[route]]
	if b == nil {
		return 0
	}
	return b.take(actions, time.Now())
}

func (c *Client) observe(route string, h http.Header) {
	policy := h.Get("RateLimit-Policy")
	if policy == "" {
		return
	}
	name := strings.Trim(strings.SplitN(policy, ";", 2)[0], `"`)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routes[route] = name
	c.buckets[name] = newBucket(name, h, time.Now())
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// call performs one request, waiting for rate-limit tokens and retrying 429 answers.
// `actions` is the number of actions the request carries (final worlds charge per action).
func (c *Client) call(ctx context.Context, method, path string, body any, actions int, out any) error {
	route := method + " " + strings.SplitN(path, "?", 2)[0]
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("encode %s: %w", route, err)
		}
	}
	rec := recorderOf(ctx)
	for attempt := 0; attempt < maxRateRetries; attempt++ {
		wait := c.reserve(route, actions)
		if err := sleepCtx(ctx, wait); err != nil {
			return err
		}
		sent := time.Now()
		status, header, data, err := c.do(ctx, method, path, payload)
		if rec != nil {
			rec.Record(exchangeOf(sent, wait, method, path, payload, status, header, data, err))
		}
		if err != nil {
			return err
		}
		c.observe(route, header)
		if status == http.StatusTooManyRequests {
			wait := defaultRetryAfter
			if ms, perr := strconv.Atoi(header.Get("Retry-After")); perr == nil && ms > 0 {
				wait = time.Duration(ms) * time.Millisecond
			}
			if err := sleepCtx(ctx, wait); err != nil {
				return err
			}
			continue
		}
		return decode(route, status, data, out)
	}
	return &Unreachable{Msg: route + ": rate limited too many times"}
}

func millis(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// exchangeOf builds the record of one request and its answer.
func exchangeOf(sent time.Time, wait time.Duration, method, path string, payload []byte, status int, header http.Header, data []byte, err error) Exchange {
	ex := Exchange{
		At: sent, Method: method, Path: path, Request: rawJSON(payload),
		Status: status, Response: rawJSON(data), Ms: millis(time.Since(sent)), WaitMs: millis(wait),
	}
	if header != nil {
		ex.Limit, ex.Left = header.Get("RateLimit-Policy"), header.Get("RateLimit-Remaining")
	}
	if err != nil {
		ex.Error = err.Error()
	}
	return ex
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte) (int, http.Header, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, reader)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		var diverged *DivergedError
		if errors.As(err, &diverged) {
			return 0, nil, nil, diverged // a replay that left its tape, not a network fault
		}
		return 0, nil, nil, &Unreachable{Msg: err.Error()}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, &Unreachable{Msg: "answer broke off: " + err.Error()}
	}
	return resp.StatusCode, resp.Header, data, nil
}

func decode(route string, status int, data []byte, out any) error {
	if status >= 502 && status <= 504 {
		return &Unreachable{Msg: fmt.Sprintf("server restarting (HTTP %d)", status)}
	}
	if status < 200 || status >= 300 {
		var e struct {
			Error string `json:"error"`
			Field string `json:"field"`
		}
		if json.Unmarshal(data, &e) != nil || e.Error == "" {
			e.Error = strings.TrimSpace(string(data))
			if len(e.Error) > 200 {
				e.Error = e.Error[:200]
			}
		}
		return &Error{Status: status, Msg: e.Error, Field: e.Field}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", route, err)
	}
	return nil
}

// Worlds lists the team's worlds.
func (c *Client) Worlds(ctx context.Context) ([]WorldSummary, error) {
	var out []WorldSummary
	return out, c.call(ctx, http.MethodGet, "/v1/worlds", nil, 0, &out)
}

// Create makes a practice world. With board=true it is the next ranking run and seed must be empty.
func (c *Client) Create(ctx context.Context, level int, seed string, board bool) (*World, error) {
	body := map[string]any{"level": level}
	if seed != "" {
		body["seed"] = seed
	}
	if board {
		body["board"] = true
	}
	var out struct {
		WorldID string `json:"worldId"`
	}
	if err := c.call(ctx, http.MethodPost, "/v1/worlds", body, 0, &out); err != nil {
		return nil, err
	}
	return c.World(out.WorldID), nil
}

// World wraps an existing world id; it makes no request.
func (c *Client) World(id string) *World { return &World{c: c, ID: id, lastSeq: -1} }

// Map returns places and the road matrix. Costs no token.
func (c *Client) Map(ctx context.Context) (*WorldMap, error) {
	var out WorldMap
	return &out, c.call(ctx, http.MethodGet, "/v1/map", nil, 0, &out)
}

// Final returns the team's copy of the final. 404 before one is set.
func (c *Client) Final(ctx context.Context) (*Final, error) {
	var out Final
	return &out, c.call(ctx, http.MethodGet, "/v1/final", nil, 0, &out)
}

// Board returns the practice rankings of a level.
func (c *Client) Board(ctx context.Context, level int) (*PracticeBoard, error) {
	var out PracticeBoard
	return &out, c.call(ctx, http.MethodGet, fmt.Sprintf("/v1/leaderboards/practice?level=%d", level), nil, 0, &out)
}
