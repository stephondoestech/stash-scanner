package app

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"stash-scanner/internal/config"
	"stash-scanner/internal/detect"
	"stash-scanner/internal/scheduler"
	"stash-scanner/internal/state"
)

func TestRunOncePersistsPendingScanOnFailure(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "scene.mp4")
	if err := os.WriteFile(filePath, []byte("media"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := config.Config{
		WatchRoots: []string{root},
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		Retry: config.Retry{
			MaxAttempts:    5,
			InitialBackoff: config.Duration{Duration: 10 * time.Second},
			MaxBackoff:     config.Duration{Duration: time.Minute},
		},
		Schedule: config.Schedule{
			Interval: config.Duration{Duration: time.Minute},
		},
	}

	store := state.NewStore(cfg.StatePath)
	now := time.Date(2026, time.March, 26, 12, 0, 0, 0, time.UTC)
	client := &fakeClient{err: errors.New("stash unavailable")}
	runner := &Runner{
		cfg:       cfg,
		logger:    log.New(io.Discard, "", 0),
		detector:  detect.New([]string{"*.mp4"}, nil),
		store:     store,
		client:    client,
		scheduler: scheduler.New(time.Minute, ""),
		now:       func() time.Time { return now },
	}

	err := runner.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected RunOnce error")
	}

	snapshot, loadErr := store.Load()
	if loadErr != nil {
		t.Fatalf("load state: %v", loadErr)
	}

	if got, want := len(snapshot.PendingScan.Paths), 1; got != want {
		t.Fatalf("pending path count mismatch: got %d want %d", got, want)
	}

	if got, want := snapshot.PendingScan.AttemptCount, 1; got != want {
		t.Fatalf("attempt count mismatch: got %d want %d", got, want)
	}

	if got, want := snapshot.PendingScan.NextAttemptAt, now.Add(10*time.Second); !got.Equal(want) {
		t.Fatalf("next attempt mismatch: got %s want %s", got, want)
	}

	if !snapshot.LastSuccessAt.IsZero() {
		t.Fatal("expected LastSuccessAt to remain zero after failure")
	}
}

func TestRunOnceDefersRetryUntilBackoffExpires(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "scene.mp4")
	if err := os.WriteFile(filePath, []byte("media"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := config.Config{
		WatchRoots: []string{root},
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		Retry: config.Retry{
			MaxAttempts:    5,
			InitialBackoff: config.Duration{Duration: 10 * time.Second},
			MaxBackoff:     config.Duration{Duration: time.Minute},
		},
		Schedule: config.Schedule{
			Interval: config.Duration{Duration: time.Minute},
		},
	}

	store := state.NewStore(cfg.StatePath)
	now := time.Date(2026, time.March, 26, 12, 0, 0, 0, time.UTC)
	snapshot := state.Snapshot{
		Paths: map[string]state.PathState{},
		PendingScan: state.PendingScan{
			Paths:         []string{"/media/previous"},
			AttemptCount:  1,
			LastError:     "stash unavailable",
			FirstFailedAt: now.Add(-5 * time.Second),
			LastFailedAt:  now.Add(-5 * time.Second),
			NextAttemptAt: now.Add(30 * time.Second),
		},
	}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	client := &fakeClient{}
	runner := &Runner{
		cfg:       cfg,
		logger:    log.New(io.Discard, "", 0),
		detector:  detect.New([]string{"*.mp4"}, nil),
		store:     store,
		client:    client,
		scheduler: scheduler.New(time.Minute, ""),
		now:       func() time.Time { return now },
	}

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if client.calls != 0 {
		t.Fatalf("expected no scan attempt during backoff, got %d calls", client.calls)
	}

	updated, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if got, want := len(updated.PendingScan.Paths), 2; got != want {
		t.Fatalf("pending path count mismatch: got %d want %d", got, want)
	}
}

type fakeClient struct {
	roots []string
	calls int
	err   error
}

func (f *fakeClient) TriggerScan(_ context.Context, _ []string) error {
	f.calls++
	return f.err
}

func (f *fakeClient) LibraryRoots(_ context.Context) ([]string, error) {
	if f.roots == nil {
		return nil, nil
	}
	return append([]string{}, f.roots...), nil
}
