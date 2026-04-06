package review

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	StashURL        string
	APIKey          string
	QueuePath       string
	Bind            string
	RefreshInterval time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		QueuePath: "data/reviewer-queue.json",
		Bind:      "127.0.0.1:8090",
	}

	cfg.StashURL = firstEnv("STASH_REVIEWER_STASH_URL", "STASH_SCANNER_STASH_URL")
	cfg.APIKey = firstEnv("STASH_REVIEWER_API_KEY", "STASH_SCANNER_API_KEY")
	if value := strings.TrimSpace(os.Getenv("STASH_REVIEWER_QUEUE_PATH")); value != "" {
		cfg.QueuePath = value
	}
	if value := strings.TrimSpace(os.Getenv("STASH_REVIEWER_BIND")); value != "" {
		cfg.Bind = value
	}
	if value := strings.TrimSpace(os.Getenv("STASH_REVIEWER_REFRESH_INTERVAL")); value != "" {
		interval, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse STASH_REVIEWER_REFRESH_INTERVAL: %w", err)
		}
		cfg.RefreshInterval = interval
	}

	if cfg.StashURL == "" {
		return Config{}, fmt.Errorf("stash URL is required")
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("api key is required")
	}
	if cfg.QueuePath == "" {
		return Config{}, fmt.Errorf("queue path is required")
	}
	if cfg.Bind == "" {
		return Config{}, fmt.Errorf("bind is required")
	}
	if cfg.RefreshInterval < 0 {
		return Config{}, fmt.Errorf("refresh interval must be zero or greater")
	}

	return cfg, nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
