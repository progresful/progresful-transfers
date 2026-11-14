package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"highwaycoins/internal/api"
)

// A run's log must give back exactly what the run sent and received: the request tape is
// what an engine change is replayed against.
func TestRunLogRoundTrip(t *testing.T) {
	root := t.TempDir()
	meta := Meta{RunID: "run-1", StartedAt: time.Now(), WorldID: "w1", Kind: "practice", Level: 8, Seed: "s1", Policy: "bench",
		Engine: "e11", EngineHash: "abc123", Params: map[string]any{"costMult": 1.3}}
	run, err := NewRun(root, meta)
	if err != nil {
		t.Fatal(err)
	}
	run.Event(json.RawMessage(`{"seq":1,"at":480,"kind":"price_request"}`))
	run.Action(480, api.Action{Type: api.ActQuote, RequestID: "R1", Prices: map[string]int{"next_day": 9000}}, api.ActionResult{OK: true, ID: "R1"})
	run.HTTP(api.Exchange{N: 1, Method: "POST", Path: "/v1/worlds/w1/advance", Status: 200, Ms: 12.5})
	run.HTTP(api.Exchange{N: 2, Method: "GET", Path: "/v1/worlds/w1/score", Status: 200, Ms: 3})
	if err := run.SaveRoster([]api.DriverProfile{{ID: "d1", HomePlaceID: "helsinki"}}); err != nil {
		t.Fatal(err)
	}
	summary := Summary{Meta: meta, EndedAt: time.Now(), Seconds: 1.5, Requests: 2, WorldEnded: true,
		Score: api.Score{MarginCents: 123456, WonOrders: 300, OnTimeShare: 0.96, Ranked: true}}
	if err := run.Finish(summary, json.RawMessage(`{"requests":[]}`)); err != nil {
		t.Fatal(err)
	}

	tape, err := ReadTape(run.Dir)
	if err != nil || len(tape) != 2 || tape[0].Path != "/v1/worlds/w1/advance" || tape[1].N != 2 || tape[0].Ms != 12.5 {
		t.Errorf("tape = %+v, %v", tape, err)
	}
	got, err := ReadMeta(run.Dir)
	if err != nil || got.RunID != "run-1" || got.Engine != "e11" || got.EngineHash != "abc123" || got.Level != 8 {
		t.Errorf("meta = %+v, %v", got, err)
	}
	for _, name := range []string{eventsFile, actionsFile, summaryFile, revealFile, rosterFile} {
		if info, err := os.Stat(filepath.Join(run.Dir, name)); err != nil || info.Size() == 0 {
			t.Errorf("%s is missing or empty (%v)", name, err)
		}
	}
	index, err := Index(root)
	if err != nil || len(index) != 1 || index[0].RunID != "run-1" || index[0].Score.MarginCents != 123456 || !index[0].WorldEnded {
		t.Errorf("index = %+v, %v", index, err)
	}
}

func TestIndexOfAnEmptyRootIsEmpty(t *testing.T) {
	index, err := Index(t.TempDir())
	if err != nil || len(index) != 0 {
		t.Errorf("no runs yet: got %d entries, %v", len(index), err)
	}
}

func TestReadMetaOfAMissingRun(t *testing.T) {
	if _, err := ReadMeta(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("a missing run folder must be an error")
	}
}
