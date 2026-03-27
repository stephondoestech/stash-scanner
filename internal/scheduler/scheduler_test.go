package scheduler

import (
	"context"
	"errors"
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

func TestRunWaitsBeforeFirstExecution(t *testing.T) {
	s := New(50*time.Millisecond, "")
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := s.Run(ctx, func(context.Context) error {
		calls++
		return nil
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error mismatch: got %v want %v", err, context.DeadlineExceeded)
	}

	if calls != 0 {
		t.Fatalf("expected no runs before first interval, got %d", calls)
	}
}
