package notes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// corpusEnv names a glob of reveal.json files, for example
//
//	NOTES_CORPUS='../../runs/*/reveal.json' go test -run Corpus ./internal/notes/
const corpusEnv = "NOTES_CORPUS"

// TestCorpusNeverPanics reads every note of every request in the given
// reveals, also of lost requests, without the safety net of Parse.
func TestCorpusNeverPanics(t *testing.T) {
	pattern := os.Getenv(corpusEnv)
	if pattern == "" {
		t.Skipf("set %s to a glob of reveal.json files", corpusEnv)
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("bad glob %q: %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no file matches %q", pattern)
	}
	type reveal struct {
		Requests []struct {
			Request struct {
				ArrivedAt int `json:"arrivedAt"`
				Note      *struct {
					Text string `json:"text"`
				} `json:"note"`
			} `json:"request"`
		} `json:"requests"`
	}
	read := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var rev reveal
		if err := json.Unmarshal(raw, &rev); err != nil {
			t.Fatalf("decode %s: %v", file, err)
		}
		for _, r := range rev.Requests {
			if r.Request.Note == nil {
				continue
			}
			text, arrived := r.Request.Note.Text, r.Request.ArrivedAt
			if got, safe := parse(text, arrived), Parse(text, arrived); got != safe {
				t.Errorf("%q: parse %+v, Parse %+v", text, got, safe)
			}
			read++
		}
	}
	t.Logf("%d notes in %d reveals read without a panic", read, len(files))
}
