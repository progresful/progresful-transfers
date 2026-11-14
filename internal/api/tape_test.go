package api

import (
	"context"
	"errors"
	"testing"
)

func TestTapeBuffersUntilAttached(t *testing.T) {
	tape := &Tape{}
	tape.Record(Exchange{Method: "POST", Path: "/v1/worlds"})
	tape.Record(Exchange{Method: "GET", Path: "/v1/worlds/w1"})
	var got []Exchange
	tape.Attach(func(ex Exchange) { got = append(got, ex) })
	tape.Record(Exchange{Method: "POST", Path: "/v1/worlds/w1/advance"})
	if len(got) != 3 || tape.Count() != 3 {
		t.Fatalf("sink saw %d exchanges, tape counts %d; want 3 and 3", len(got), tape.Count())
	}
	for i, ex := range got {
		if ex.N != i+1 {
			t.Errorf("exchange %d has n=%d", i, ex.N)
		}
	}
}

func replayClient(t *testing.T, exchanges []Exchange) *Client {
	t.Helper()
	c, err := NewClient("http://replay.invalid", "key")
	if err != nil {
		t.Fatal(err)
	}
	c.SetTransport(NewReplayTransport(exchanges))
	return c
}

func TestReplayServesRecordedAnswers(t *testing.T) {
	c := replayClient(t, []Exchange{
		{N: 1, Method: "POST", Path: "/v1/worlds", Request: rawJSON([]byte(`{"level":8,"seed":"s"}`)), Status: 201, Response: rawJSON([]byte(`{"worldId":"w1"}`))},
		{N: 2, Method: "GET", Path: "/v1/worlds/w1/score", Status: 200, Response: rawJSON([]byte(`{"marginCents":1234,"ranked":true}`))},
	})
	world, err := c.Create(context.Background(), 8, "s", false)
	if err != nil || world.ID != "w1" {
		t.Fatalf("Create = %v, %v", world, err)
	}
	score, err := world.Score(context.Background())
	if err != nil || score.MarginCents != 1234 || !score.Ranked {
		t.Fatalf("Score = %+v, %v", score, err)
	}
}

func TestReplayReportsFirstDifferentRequest(t *testing.T) {
	c := replayClient(t, []Exchange{
		{N: 1, Method: "POST", Path: "/v1/worlds", Request: rawJSON([]byte(`{"level":8,"seed":"s"}`)), Status: 201, Response: rawJSON([]byte(`{"worldId":"w1"}`))},
	})
	_, err := c.Create(context.Background(), 7, "s", false)
	var diverged *DivergedError
	if !errors.As(err, &diverged) || diverged.N != 1 {
		t.Fatalf("a different body should diverge at request 1, got %v", err)
	}
	if IsUnreachable(err) {
		t.Error("a diverged replay must not look like a network fault")
	}
}

func TestRecorderSeesEveryExchange(t *testing.T) {
	c := replayClient(t, []Exchange{
		{N: 1, Method: "GET", Path: "/v1/worlds", Status: 200, Response: rawJSON([]byte(`[]`))},
	})
	tape := &Tape{}
	var seen []Exchange
	tape.Attach(func(ex Exchange) { seen = append(seen, ex) })
	if _, err := c.Worlds(WithRecorder(context.Background(), tape)); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].Method != "GET" || seen[0].Path != "/v1/worlds" || seen[0].Status != 200 || string(seen[0].Response) != "[]" {
		t.Fatalf("recorded %+v", seen)
	}
}

func TestRawJSONWrapsPlainText(t *testing.T) {
	if got := string(rawJSON([]byte("Bad Gateway"))); got != `"Bad Gateway"` {
		t.Errorf("plain text is stored as %s, want a JSON string", got)
	}
	if rawJSON(nil) != nil {
		t.Error("an empty body is stored as nothing")
	}
}
