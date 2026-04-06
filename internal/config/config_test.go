package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFile(t *testing.T) {
	t.Setenv("STASH_SCANNER_STASH_URL", "")
	t.Setenv("STASH_SCANNER_API_KEY", "")
	t.Setenv("STASH_SCANNER_STATE_PATH", "")
	t.Setenv("STASH_SCANNER_CONTROL_BIND", "")
	t.Setenv("STASH_SCANNER_CONTROL_FALLBACK_BIND", "")
	t.Setenv("STASH_SCANNER_WATCH_ROOTS_FROM_STASH", "")
	t.Setenv("STASH_SCANNER_DRY_RUN", "")
	t.Setenv("STASH_SCANNER_DEBUG", "")
	t.Setenv("STASH_SCANNER_POST_SCAN_TASKS", "")
	t.Setenv("STASH_SCANNER_IDENTIFY_STASH_BOX_INDEXES", "")
	t.Setenv("STASH_SCANNER_IDENTIFY_STASH_BOX_ENDPOINTS", "")
	t.Setenv("STASH_SCANNER_IDENTIFY_SCRAPER_IDS", "")
	t.Setenv("STASH_SCANNER_POST_SCAN_CLEAN_DRY_RUN", "")
	t.Setenv("STASH_SCANNER_DEBOUNCE_WINDOW", "")
	t.Setenv("STASH_SCANNER_INTERVAL", "")
	t.Setenv("STASH_SCANNER_DAILY_TIME", "")
	t.Setenv("STASH_SCANNER_WATCH_ROOTS", "")
	t.Setenv("STASH_SCANNER_INCLUDE_PATTERNS", "")
	t.Setenv("STASH_SCANNER_EXCLUDE_PATTERNS", "")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	data := []byte(`{
  "stash_url": "http://localhost:9999",
  "api_key": "secret",
  "watch_roots": ["./library"],
  "state_path": "data/state.json",
  "debounce_window": "45s",
  "dry_run": true,
  "schedule": {
    "interval": "10m",
    "daily_time": ""
  }
}`)

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got, want := cfg.WatchRoots[0], "library"; got != want {
		t.Fatalf("watch root mismatch: got %q want %q", got, want)
	}

	if got, want := cfg.Schedule.Interval.Duration, 10*time.Minute; got != want {
		t.Fatalf("interval mismatch: got %s want %s", got, want)
	}
}

func TestLoadConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("STASH_SCANNER_WATCH_ROOTS", "/tmp/media,/tmp/other")
	t.Setenv("STASH_SCANNER_WATCH_ROOTS_FROM_STASH", "true")
	t.Setenv("STASH_SCANNER_STASH_URL", "http://localhost:9999")
	t.Setenv("STASH_SCANNER_API_KEY", "secret")
	t.Setenv("STASH_SCANNER_INTERVAL", "20m")
	t.Setenv("STASH_SCANNER_DAILY_TIME", "")
	t.Setenv("STASH_SCANNER_CONTROL_FALLBACK_BIND", "127.0.0.1:8089")
	t.Setenv("STASH_SCANNER_RETRY_MAX_ATTEMPTS", "7")
	t.Setenv("STASH_SCANNER_RETRY_INITIAL_BACKOFF", "45s")
	t.Setenv("STASH_SCANNER_RETRY_MAX_BACKOFF", "10m")
	t.Setenv("STASH_SCANNER_DEBUG", "true")
	t.Setenv("STASH_SCANNER_POST_SCAN_TASKS", "auto_tag,identify,clean")
	t.Setenv("STASH_SCANNER_IDENTIFY_STASH_BOX_INDEXES", "0,2")
	t.Setenv("STASH_SCANNER_POST_SCAN_CLEAN_DRY_RUN", "false")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.WatchRoots) != 2 {
		t.Fatalf("expected 2 watch roots, got %d", len(cfg.WatchRoots))
	}

	if got, want := cfg.Schedule.Interval.Duration, 20*time.Minute; got != want {
		t.Fatalf("interval mismatch: got %s want %s", got, want)
	}

	if got, want := cfg.Retry.MaxAttempts, 7; got != want {
		t.Fatalf("retry max attempts mismatch: got %d want %d", got, want)
	}

	if got, want := cfg.Control.FallbackBind, "127.0.0.1:8089"; got != want {
		t.Fatalf("fallback bind mismatch: got %q want %q", got, want)
	}

	if !cfg.WatchRootsFromStash {
		t.Fatal("expected watch roots from stash to be enabled")
	}

	if !cfg.Debug {
		t.Fatal("expected debug mode to be enabled")
	}

	if got, want := len(cfg.PostScan.Tasks), 3; got != want {
		t.Fatalf("post scan task count mismatch: got %d want %d", got, want)
	}

	if got, want := cfg.PostScan.Tasks[0], "identify"; got != want {
		t.Fatalf("post scan task order[0] mismatch: got %q want %q", got, want)
	}

	if got, want := cfg.PostScan.Tasks[1], "auto_tag"; got != want {
		t.Fatalf("post scan task order[1] mismatch: got %q want %q", got, want)
	}

	if got, want := cfg.PostScan.Tasks[2], "clean"; got != want {
		t.Fatalf("post scan task order[2] mismatch: got %q want %q", got, want)
	}

	if got, want := len(cfg.PostScan.IdentifyStashBoxIndexes), 2; got != want {
		t.Fatalf("identify source count mismatch: got %d want %d", got, want)
	}
}

func TestValidateRejectsConflictingScheduleModes(t *testing.T) {
	cfg := Config{
		WatchRoots: []string{"/tmp/media"},
		StatePath:  "data/state.json",
		Schedule: Schedule{
			Interval:  Duration{Duration: 15 * time.Minute},
			DailyTime: "03:00",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected schedule validation error")
	}
}

func TestValidateAllowsStashWatchRootsWithoutLocalPaths(t *testing.T) {
	cfg := Config{
		StashURL:            "http://localhost:9999",
		APIKey:              "secret",
		WatchRootsFromStash: true,
		StatePath:           "data/state.json",
		Retry: Retry{
			MaxAttempts:    1,
			InitialBackoff: Duration{Duration: 10 * time.Second},
			MaxBackoff:     Duration{Duration: time.Minute},
		},
		Schedule: Schedule{
			Interval: Duration{Duration: 15 * time.Minute},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsInvalidRetrySettings(t *testing.T) {
	cfg := Config{
		WatchRoots: []string{"/tmp/media"},
		StatePath:  "data/state.json",
		Retry: Retry{
			MaxAttempts:    0,
			InitialBackoff: Duration{Duration: 10 * time.Second},
			MaxBackoff:     Duration{Duration: time.Minute},
		},
		Schedule: Schedule{
			Interval: Duration{Duration: 15 * time.Minute},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected retry validation error")
	}
}

func TestValidateRejectsIdentifyTaskWithoutSources(t *testing.T) {
	cfg := Config{
		WatchRoots: []string{"/tmp/media"},
		StatePath:  "data/state.json",
		StashURL:   "",
		APIKey:     "",
		PostScan: PostScan{
			Tasks: []string{"identify"},
		},
		Retry: Retry{
			MaxAttempts:    1,
			InitialBackoff: Duration{Duration: 10 * time.Second},
			MaxBackoff:     Duration{Duration: time.Minute},
		},
		Schedule: Schedule{
			Interval: Duration{Duration: 15 * time.Minute},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected identify source validation error")
	}
}

func TestValidateAllowsIdentifyTaskWithoutLocalSourcesWhenStashDiscoveryIsAvailable(t *testing.T) {
	cfg := Config{
		WatchRoots: []string{"/tmp/media"},
		StatePath:  "data/state.json",
		StashURL:   "http://localhost:9999",
		APIKey:     "secret",
		PostScan: PostScan{
			Tasks: []string{"identify"},
		},
		Retry: Retry{
			MaxAttempts:    1,
			InitialBackoff: Duration{Duration: 10 * time.Second},
			MaxBackoff:     Duration{Duration: time.Minute},
		},
		Schedule: Schedule{
			Interval: Duration{Duration: 15 * time.Minute},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
