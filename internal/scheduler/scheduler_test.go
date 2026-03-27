package scheduler

import (
	"testing"
	"time"
)

func TestNextWaitUsesIntervalWhenConfigured(t *testing.T) {
	s := New(15*time.Minute, "")

	wait, err := s.nextWait(time.Date(2026, time.March, 26, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("nextWait: %v", err)
	}

	if got, want := wait, 15*time.Minute; got != want {
		t.Fatalf("wait mismatch: got %s want %s", got, want)
	}
}

func TestNextWaitUsesNextDailyTime(t *testing.T) {
	s := New(0, "03:00")
	now := time.Date(2026, time.March, 26, 2, 30, 0, 0, time.UTC)

	wait, err := s.nextWait(now)
	if err != nil {
		t.Fatalf("nextWait: %v", err)
	}

	if got, want := wait, 30*time.Minute; got != want {
		t.Fatalf("wait mismatch: got %s want %s", got, want)
	}
}

func TestNextWaitRollsToTomorrowWhenDailyTimePassed(t *testing.T) {
	s := New(0, "03:00")
	now := time.Date(2026, time.March, 26, 4, 0, 0, 0, time.UTC)

	wait, err := s.nextWait(now)
	if err != nil {
		t.Fatalf("nextWait: %v", err)
	}

	if got, want := wait, 23*time.Hour; got != want {
		t.Fatalf("wait mismatch: got %s want %s", got, want)
	}
}
