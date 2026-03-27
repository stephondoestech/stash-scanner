package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	StashURL            string   `json:"stash_url"`
	APIKey              string   `json:"api_key"`
	WatchRoots          []string `json:"watch_roots"`
	WatchRootsFromStash bool     `json:"watch_roots_from_stash"`
	IncludePatterns     []string `json:"include_patterns"`
	ExcludePatterns     []string `json:"exclude_patterns"`
	StatePath           string   `json:"state_path"`
	DebounceWindow      Duration `json:"debounce_window"`
	DryRun              bool     `json:"dry_run"`
	Debug               bool     `json:"debug"`
	Control             Control  `json:"control"`
	Retry               Retry    `json:"retry"`
	Schedule            Schedule `json:"schedule"`
}

type Schedule struct {
	Interval  Duration `json:"interval"`
	DailyTime string   `json:"daily_time"`
}

type Retry struct {
	MaxAttempts    int      `json:"max_attempts"`
	InitialBackoff Duration `json:"initial_backoff"`
	MaxBackoff     Duration `json:"max_backoff"`
}

type Control struct {
	Bind         string `json:"bind"`
	FallbackBind string `json:"fallback_bind"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}

	d.Duration = parsed
	return nil
}

func Load(path string) (Config, error) {
	loadDotEnv(".env")

	cfg := defaultConfig()
	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return Config{}, err
		}
	}

	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		StatePath:      "data/state.json",
		DebounceWindow: Duration{Duration: 30 * time.Second},
		DryRun:         true,
		Debug:          false,
		Control: Control{
			Bind: "127.0.0.1:8088",
		},
		Retry: Retry{
			MaxAttempts:    5,
			InitialBackoff: Duration{Duration: 30 * time.Second},
			MaxBackoff:     Duration{Duration: 15 * time.Minute},
		},
		Schedule: Schedule{
			Interval: Duration{Duration: 15 * time.Minute},
		},
	}
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}

	return nil
}

func applyEnv(cfg *Config) {
	overrideString(&cfg.StashURL, "STASH_SCANNER_STASH_URL")
	overrideString(&cfg.APIKey, "STASH_SCANNER_API_KEY")
	overrideString(&cfg.StatePath, "STASH_SCANNER_STATE_PATH")
	overrideString(&cfg.Control.Bind, "STASH_SCANNER_CONTROL_BIND")
	overrideString(&cfg.Control.FallbackBind, "STASH_SCANNER_CONTROL_FALLBACK_BIND")
	overrideBool(&cfg.WatchRootsFromStash, "STASH_SCANNER_WATCH_ROOTS_FROM_STASH")
	overrideBool(&cfg.DryRun, "STASH_SCANNER_DRY_RUN")
	overrideBool(&cfg.Debug, "STASH_SCANNER_DEBUG")
	overrideDuration(&cfg.DebounceWindow.Duration, "STASH_SCANNER_DEBOUNCE_WINDOW")
	overrideInt(&cfg.Retry.MaxAttempts, "STASH_SCANNER_RETRY_MAX_ATTEMPTS")
	overrideDuration(&cfg.Retry.InitialBackoff.Duration, "STASH_SCANNER_RETRY_INITIAL_BACKOFF")
	overrideDuration(&cfg.Retry.MaxBackoff.Duration, "STASH_SCANNER_RETRY_MAX_BACKOFF")
	overrideDuration(&cfg.Schedule.Interval.Duration, "STASH_SCANNER_INTERVAL")
	overrideString(&cfg.Schedule.DailyTime, "STASH_SCANNER_DAILY_TIME")
	overrideSlice(&cfg.WatchRoots, "STASH_SCANNER_WATCH_ROOTS")
	overrideSlice(&cfg.IncludePatterns, "STASH_SCANNER_INCLUDE_PATTERNS")
	overrideSlice(&cfg.ExcludePatterns, "STASH_SCANNER_EXCLUDE_PATTERNS")
}

func (c *Config) Validate() error {
	if !c.WatchRootsFromStash && len(c.WatchRoots) == 0 {
		return fmt.Errorf("watch_roots is required unless watch_roots_from_stash is true")
	}

	for i, root := range c.WatchRoots {
		if root == "" {
			return fmt.Errorf("watch_roots[%d] is empty", i)
		}
		c.WatchRoots[i] = filepath.Clean(root)
	}

	hasInterval := c.Schedule.Interval.Duration > 0
	hasDaily := strings.TrimSpace(c.Schedule.DailyTime) != ""
	if hasInterval == hasDaily {
		return fmt.Errorf("configure exactly one of schedule.interval or schedule.daily_time")
	}

	if hasDaily {
		if _, err := time.Parse("15:04", c.Schedule.DailyTime); err != nil {
			return fmt.Errorf("schedule.daily_time must use HH:MM: %w", err)
		}
	}

	if strings.TrimSpace(c.StatePath) == "" {
		return fmt.Errorf("state_path is required")
	}

	if c.WatchRootsFromStash {
		if strings.TrimSpace(c.StashURL) == "" {
			return fmt.Errorf("stash_url is required when watch_roots_from_stash is true")
		}
		if strings.TrimSpace(c.APIKey) == "" {
			return fmt.Errorf("api_key is required when watch_roots_from_stash is true")
		}
	}

	if c.Retry.MaxAttempts < 1 {
		return fmt.Errorf("retry.max_attempts must be at least 1")
	}

	if c.Retry.InitialBackoff.Duration <= 0 {
		return fmt.Errorf("retry.initial_backoff must be greater than zero")
	}

	if c.Retry.MaxBackoff.Duration < c.Retry.InitialBackoff.Duration {
		return fmt.Errorf("retry.max_backoff must be greater than or equal to retry.initial_backoff")
	}

	return nil
}

func overrideString(target *string, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		*target = value
	}
}

func overrideBool(target *bool, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}

	parsed, err := strconv.ParseBool(value)
	if err == nil {
		*target = parsed
	}
}

func overrideInt(target *int, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}

	parsed, err := strconv.Atoi(value)
	if err == nil {
		*target = parsed
	}
}

func overrideDuration(target *time.Duration, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}

	parsed, err := time.ParseDuration(value)
	if err == nil {
		*target = parsed
	}
}

func overrideSlice(target *[]string, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	*target = items
}

func loadDotEnv(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}
