package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
)

// The public feed of a final or test final (GET /v1/leaderboard, server-sent events,
// no key): about every two seconds the standing of every team.

// FeedTeam is one team's standing in the live feed.
type FeedTeam struct {
	Rank             *int    `json:"rank"`
	Team             string  `json:"team"`
	Name             string  `json:"name"`
	MarginCents      int     `json:"marginCents"`
	RevenueCents     int     `json:"revenueCents"`
	MarginShare      float64 `json:"marginShare"`
	WonOrders        int     `json:"wonOrders"`
	OnTimeShare      float64 `json:"onTimeShare"`
	UndeliveredShare float64 `json:"undeliveredShare"`
	Ranked           bool    `json:"ranked"`
}

// FinalFeed is one message of the live feed. Status is none, scheduled, running or ended.
type FinalFeed struct {
	Status   string     `json:"status"`
	Now      int        `json:"now"`
	EndsAt   int        `json:"endsAt"`
	StartsAt string     `json:"startsAt"`
	Teams    []FeedTeam `json:"teams"`
}

const (
	feedPath      = "/v1/leaderboard"
	feedDataField = "data:"
	// feedLineMax bounds one message: a few hundred teams fit many times over.
	feedLineMax = 8 << 20
)

// StreamLeaderboard reads the public live feed and hands each message, as sent, to
// handle until the context ends, the server closes the stream, or handle returns an error.
func StreamLeaderboard(ctx context.Context, baseURL string, handle func(raw []byte) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+feedPath, nil)
	if err != nil {
		return fmt.Errorf("build feed request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Unreachable{Msg: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Error{Status: resp.StatusCode, Msg: "live feed refused"}
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1<<16), feedLineMax)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte(feedDataField)) {
			continue
		}
		payload := bytes.TrimSpace(line[len(feedDataField):])
		if len(payload) == 0 {
			continue
		}
		if err := handle(append([]byte(nil), payload...)); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return &Unreachable{Msg: "feed broke off: " + err.Error()}
	}
	return nil
}
