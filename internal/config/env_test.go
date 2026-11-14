package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	cases := []struct {
		name             string
		file             string
		envKey, envURL   string
		wantKey, wantURL string
		wantErr          bool
	}{
		{"key from the file", "SPREAD_KEY=file-key\n", "", "", "file-key", DefaultURL, false},
		{"only the active line counts", "# team: one\n# SPREAD_KEY=old-key\n\n# team: two\nSPREAD_KEY=new-key\n", "", "", "new-key", DefaultURL, false},
		{"quotes and export are accepted", "export SPREAD_KEY=\"quoted-key\"\nSPREAD_URL='http://localhost:9'\n", "", "", "quoted-key", "http://localhost:9", false},
		{"the environment wins over the file", "SPREAD_KEY=file-key\n", "env-key", "http://env:1", "env-key", "http://env:1", false},
		{"no key anywhere", "# SPREAD_KEY=commented\n", "", "", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("SPREAD_KEY", c.envKey)
			t.Setenv("SPREAD_URL", c.envURL)
			cfg, err := Load(writeEnv(t, c.file))
			if (err != nil) != c.wantErr {
				t.Fatalf("error = %v, want error %v", err, c.wantErr)
			}
			if err == nil && (cfg.Key != c.wantKey || cfg.URL != c.wantURL) {
				t.Errorf("got key %q url %q, want %q and %q", cfg.Key, cfg.URL, c.wantKey, c.wantURL)
			}
		})
	}
}

func TestLoadWithoutAFile(t *testing.T) {
	t.Setenv("SPREAD_KEY", "env-key")
	t.Setenv("SPREAD_URL", "")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil || cfg.Key != "env-key" || cfg.URL != DefaultURL {
		t.Errorf("a missing file is fine when the key is exported: %+v, %v", cfg, err)
	}
}
