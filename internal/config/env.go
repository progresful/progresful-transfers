// Package config loads the server address and team key from the environment or a .env file.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultURL is the organizer's server.
	DefaultURL = "https://spread.bahnexpress.com"
	envURL     = "SPREAD_URL"
	envKey     = "SPREAD_KEY"
)

// Config is what the client needs to reach the server.
type Config struct {
	URL string
	Key string
}

// Load reads SPREAD_URL and SPREAD_KEY from the environment, falling back to the
// given .env file for values that are not exported. The key is never logged.
func Load(envFile string) (Config, error) {
	file, err := readEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}
	pick := func(name, fallback string) string {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
		if v := file[name]; v != "" {
			return v
		}
		return fallback
	}
	cfg := Config{URL: pick(envURL, DefaultURL), Key: pick(envKey, "")}
	if cfg.Key == "" {
		return Config{}, fmt.Errorf("%s is not set: export it or put it in %s", envKey, envFile)
	}
	return cfg, nil
}

func readEnvFile(path string) (map[string]string, error) {
	values := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(name)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return values, nil
}
