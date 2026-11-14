package ui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeRun(t *testing.T, root, id string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

const (
	testEvents = `{"seq":1,"at":555,"kind":"order_won","order":{"id":"R1"}}
{"seq":2,"at":556,"kind":"gig_viewed","gigId":"g1","driverId":"d1"}
{"seq":3,"at":600,"kind":"gig_booked","gigId":"g1","driverId":"d1"}
`
	testActions = `{"at":540,"action":{"type":"quote","requestId":"R1"},"result":{"ok":true}}
{"at":555,"action":{"type":"post","stops":[{"orderId":"R1","leg":0,"kind":"pickup"}]},"result":{"ok":true,"id":"g1"}}
`
)

// A finished run is sent whole and then closed: its actions first, its events after, a
// driver's look at a gig in the short form.
func TestStreamSendsAFinishedRun(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, "20261114-100000-L8-a-1", map[string]string{
		metaFile: `{"runId":"20261114-100000-L8-a-1"}`, summaryFile: `{"score":{}}`, eventsFile: testEvents, actionsFile: testActions,
	})
	rec := get(t, (&Server{Root: root}).Handler(), "/api/runs/20261114-100000-L8-a-1/stream")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var lines []string
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	want := []string{
		`data: {"a":{"at":540,"action":{"type":"quote","requestId":"R1"},"result":{"ok":true}}}`,
		`data: {"a":{"at":555,"action":{"type":"post","stops":[{"orderId":"R1","leg":0,"kind":"pickup"}]},"result":{"ok":true,"id":"g1"}}}`,
		`data: {"e":{"seq":1,"at":555,"kind":"order_won","order":{"id":"R1"}}}`,
		`data: {"v":[556,"g1"]}`,
		`data: {"e":{"seq":3,"at":600,"kind":"gig_booked","gigId":"g1","driverId":"d1"}}`,
		`event: end`,
		`data: {}`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("stream:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// A run that is being played is followed: lines written later arrive, a half-written
// line waits for its end.
func TestStreamFollowsARunBeingPlayed(t *testing.T) {
	root := t.TempDir()
	id := "20261114-100000-live-L8-b"
	writeRun(t, root, id, map[string]string{metaFile: `{"runId":"x"}`, eventsFile: `{"at":1,"kind":"order_lost"}` + "\n" + `{"at":2,"kind":"ord`})
	srv := httptest.NewServer((&Server{Root: root}).Handler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/runs/"+id+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "data:") || strings.HasPrefix(l, "event:") {
				got <- l
			}
		}
		close(got)
	}()
	next := func() string {
		select {
		case l := <-got:
			return l
		case <-ctx.Done():
			t.Fatal("the stream sent nothing in time")
			return ""
		}
	}
	if l := next(); l != `data: {"e":{"at":1,"kind":"order_lost"}}` {
		t.Fatalf("first line %q", l)
	}
	f, err := os.OpenFile(filepath.Join(root, id, eventsFile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`er_won"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if l := next(); l != `data: {"e":{"at":2,"kind":"order_won"}}` {
		t.Fatalf("the completed line arrived as %q", l)
	}
	if err := os.WriteFile(filepath.Join(root, id, summaryFile), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if l := next(); l != "event: end" {
		t.Fatalf("after the summary the stream should end, got %q", l)
	}
}

// The run list is newest first; a run without a summary is live only while it writes.
func TestRunsListAndLiveFlag(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, "20261114-090000-L8-old-1", map[string]string{metaFile: `{"runId":"a"}`, summaryFile: `{"score":{"marginCents":5},"worldEnded":true}`, eventsFile: ""})
	writeRun(t, root, "20261114-100000-L8-stopped-2", map[string]string{metaFile: `{"runId":"b"}`, eventsFile: ""})
	writeRun(t, root, "20261114-110000-live-L8-now", map[string]string{metaFile: `{"runId":"c"}`, eventsFile: ""})
	stale := time.Now().Add(-10 * time.Minute)
	for _, name := range []string{eventsFile, metaFile} {
		if err := os.Chtimes(filepath.Join(root, "20261114-100000-L8-stopped-2", name), stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := get(t, (&Server{Root: root}).Handler(), "/api/runs").Body.String()
	now, stopped, old := strings.Index(body, "live-L8-now"), strings.Index(body, "stopped-2"), strings.Index(body, "old-1")
	if now < 0 || stopped < now || old < stopped {
		t.Fatalf("runs should come newest first: %s", body)
	}
	if !strings.Contains(body, `"id":"20261114-110000-live-L8-now","live":true`) || !strings.Contains(body, `"id":"20261114-100000-L8-stopped-2","live":false`) || !strings.Contains(body, `"id":"20261114-090000-L8-old-1","live":false,"complete":true`) {
		t.Errorf("live flags: %s", body)
	}
}

// Only a run folder of this root is served.
func TestRunPathsStayInsideTheRoot(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, "20261114-100000-L8-a-1", map[string]string{metaFile: `{}`, rosterFile: `[{"id":"d1","homePlaceId":"oulu"}]`})
	h := (&Server{Root: root}).Handler()
	if rec := get(t, h, "/api/runs/20261114-100000-L8-a-1/roster"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "oulu") {
		t.Errorf("roster of a run: status %d, body %q", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/api/runs/..%2F..%2Fetc/roster", "/api/runs/nope/stream", "/api/runs/.hidden/roster"} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
	if rec := get(t, h, "/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>Progresful</title>") {
		t.Errorf("the page: status %d", rec.Code)
	}
	if rec := get(t, h, "/api/config"); !strings.Contains(rec.Body.String(), `"team":""`) {
		t.Errorf("config: %q", rec.Body.String())
	}
}
