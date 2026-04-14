package review

import "testing"

func TestLoadConfigReadsMatchThresholdOverrides(t *testing.T) {
	t.Setenv("STASH_REVIEWER_STASH_URL", "http://stash.local")
	t.Setenv("STASH_REVIEWER_API_KEY", "secret")
	t.Setenv("STASH_REVIEWER_MIN_SCORE", "11")
	t.Setenv("STASH_REVIEWER_MIN_LEAD", "4")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got, want := cfg.MinScore, 11; got != want {
		t.Fatalf("min score mismatch: got %d want %d", got, want)
	}
	if got, want := cfg.MinLead, 4; got != want {
		t.Fatalf("min lead mismatch: got %d want %d", got, want)
	}
}

func TestLoadConfigRejectsInvalidMatchThresholds(t *testing.T) {
	t.Setenv("STASH_REVIEWER_STASH_URL", "http://stash.local")
	t.Setenv("STASH_REVIEWER_API_KEY", "secret")
	t.Setenv("STASH_REVIEWER_MIN_SCORE", "0")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected min score validation error")
	}
}
