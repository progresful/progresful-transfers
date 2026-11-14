package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// World is one world of the team: a practice world, a fork, or the final copy.
type World struct {
	c       *Client
	ID      string
	lastSeq int
}

func (w *World) path(tail string) string { return "/v1/worlds/" + w.ID + tail }

// Client returns the client the world belongs to.
func (w *World) Client() *Client { return w.c }

// Snapshot reads the current desk state.
func (w *World) Snapshot(ctx context.Context) (*Snapshot, error) {
	var out Snapshot
	return &out, w.c.call(ctx, http.MethodGet, w.path(""), nil, 0, &out)
}

// Act sends actions in batches of 50; results come back in action order.
func (w *World) Act(ctx context.Context, actions []Action) ([]ActionResult, error) {
	results := make([]ActionResult, 0, len(actions))
	for i := 0; i < len(actions); i += MaxActionsPerCall {
		batch := actions[i:min(i+MaxActionsPerCall, len(actions))]
		var out []ActionResult
		if err := w.c.call(ctx, http.MethodPost, w.path("/actions"), batch, len(batch), &out); err != nil {
			return results, fmt.Errorf("actions: %w", err)
		}
		if len(out) != len(batch) {
			return results, fmt.Errorf("actions: %d results for %d actions", len(out), len(batch))
		}
		results = append(results, out...)
	}
	return results, nil
}

// AdvanceUntil applies actions at the current minute, then moves the clock to the first
// event of one of the kinds, but not past max. Practice worlds only. Repeated events
// (after a server restart) are dropped by seq.
func (w *World) AdvanceUntil(ctx context.Context, kinds []string, max int, actions []Action) (*Advanced, error) {
	var early []ActionResult
	if len(actions) > MaxActionsPerCall {
		head := actions[:len(actions)-MaxActionsPerCall]
		res, err := w.Act(ctx, head)
		if err != nil {
			return nil, err
		}
		early, actions = res, actions[len(head):]
	}
	body := map[string]any{"untilEvent": kinds, "max": max}
	if len(kinds) == 0 {
		body = map[string]any{"to": max}
	}
	if len(actions) > 0 {
		body["actions"] = actions
	}
	var step Advanced
	if err := w.c.call(ctx, http.MethodPost, w.path("/advance"), body, len(actions), &step); err != nil {
		return nil, fmt.Errorf("advance: %w", err)
	}
	if len(step.Results) != len(actions) {
		return nil, fmt.Errorf("advance: %d results for %d actions", len(step.Results), len(actions))
	}
	fresh := step.Events[:0]
	for _, e := range step.Events {
		if e.Seq > w.lastSeq {
			fresh = append(fresh, e)
			w.lastSeq = e.Seq
		}
	}
	step.Events = fresh
	step.Results = append(early, step.Results...)
	return &step, nil
}

// Events returns events with seq above `after`, oldest first. limit 0 means no limit.
func (w *World) Events(ctx context.Context, after, limit int) ([]Event, error) {
	q := fmt.Sprintf("/events?after=%d", after)
	if limit > 0 {
		q += fmt.Sprintf("&limit=%d", limit)
	}
	var out []Event
	return out, w.c.call(ctx, http.MethodGet, w.path(q), nil, 0, &out)
}

// Roster returns every driver's public profile.
func (w *World) Roster(ctx context.Context) ([]DriverProfile, error) {
	var out []DriverProfile
	return out, w.c.call(ctx, http.MethodGet, w.path("/drivers"), nil, 0, &out)
}

// Score reads the current score.
func (w *World) Score(ctx context.Context) (*Score, error) {
	var out Score
	return &out, w.c.call(ctx, http.MethodGet, w.path("/score"), nil, 0, &out)
}

// Fork copies a practice world at its current minute.
func (w *World) Fork(ctx context.Context) (*World, error) {
	var out struct {
		WorldID string `json:"worldId"`
	}
	if err := w.c.call(ctx, http.MethodPost, w.path("/fork"), nil, 0, &out); err != nil {
		return nil, err
	}
	return w.c.World(out.WorldID), nil
}

// RevealRaw returns the reveal of an ended world as raw JSON (for the run log).
func (w *World) RevealRaw(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, w.c.call(ctx, http.MethodGet, w.path("/reveal"), nil, 0, &out)
}

// Delete removes a practice world. A ranking run first plays to its end and still counts.
func (w *World) Delete(ctx context.Context) error {
	return w.c.call(ctx, http.MethodDelete, w.path(""), nil, 0, nil)
}
