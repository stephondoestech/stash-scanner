package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	dotenv := []byte("STASH_SCANNER_STASH_URL=http://localhost:9999\nSTASH_SCANNER_API_KEY=secret\nSTASH_SCANNER_WATCH_ROOTS_FROM_STASH=true\n")
	if err := os.WriteFile(filepath.Join(dir, ".env"), dotenv, 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfgPath := filepath.Join(dir, "config.json")
	configBody := []byte(`{
  "watch_roots": [],
  "state_path": "data/state.json",
  "retry": {
    "max_attempts": 1,
    "initial_backoff": "10s",
    "max_backoff": "1m"
  },
  "schedule": {
    "interval": "15m",
    "daily_time": ""
  }
}`)
	if err := os.WriteFile(cfgPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.StashURL, "http://localhost:9999"; got != want {
		t.Fatalf("stash url mismatch: got %q want %q", got, want)
	}

	if !cfg.WatchRootsFromStash {
		t.Fatal("expected watch roots from stash from .env")
	}

	if got, want := cfg.Schedule.Interval.Duration, 15*time.Minute; got != want {
		t.Fatalf("interval mismatch: got %s want %s", got, want)
	}
}
