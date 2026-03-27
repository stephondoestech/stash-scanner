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
	"stash-scanner/internal/stash"
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
		pollEvery: time.Millisecond,
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
		pollEvery: time.Millisecond,
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

func TestRunOnceWaitsForStashJobCompletion(t *testing.T) {
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
	client := &fakeClient{
		jobID: "job-123",
		jobs: []stash.Job{
			{ID: "job-123", Status: "RUNNING", Description: "Scanning", Progress: 0.25},
			{ID: "job-123", Status: "FINISHED", Description: "Scanning", Progress: 1},
		},
	}
	runner := &Runner{
		cfg:       cfg,
		logger:    log.New(io.Discard, "", 0),
		detector:  detect.New([]string{"*.mp4"}, nil),
		store:     store,
		client:    client,
		scheduler: scheduler.New(time.Minute, ""),
		now:       func() time.Time { return time.Date(2026, time.March, 27, 1, 0, 0, 0, time.UTC) },
		pollEvery: time.Millisecond,
	}

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	status, err := runner.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if got, want := status.LastRun.StashTask.Status, "FINISHED"; got != want {
		t.Fatalf("stash task status mismatch: got %q want %q", got, want)
	}

	if client.findCalls < 2 {
		t.Fatalf("expected repeated job polling, got %d polls", client.findCalls)
	}
}

func TestStopActiveRunStopsRemoteJob(t *testing.T) {
	client := &fakeClient{}
	runner := &Runner{
		logger:  log.New(io.Discard, "", 0),
		client:  client,
		now:     func() time.Time { return time.Date(2026, time.March, 27, 1, 0, 0, 0, time.UTC) },
		running: true,
		currentRun: RunState{
			Trigger: "manual",
			StashTask: StashTaskStatus{
				ID: "job-999",
			},
		},
		cancelRun: func() {
			client.cancelled = true
		},
	}

	if err := runner.StopActiveRun(context.Background()); err != nil {
		t.Fatalf("StopActiveRun: %v", err)
	}

	if !client.cancelled {
		t.Fatal("expected local cancel to be invoked")
	}

	if got, want := client.stoppedJobID, "job-999"; got != want {
		t.Fatalf("stopped job mismatch: got %q want %q", got, want)
	}
}

type fakeClient struct {
	roots        []string
	calls        int
	err          error
	jobID        string
	jobs         []stash.Job
	findCalls    int
	stopErr      error
	stoppedJobID string
	cancelled    bool
}

func (f *fakeClient) TriggerScan(_ context.Context, _ []string) (string, error) {
	f.calls++
	return f.jobID, f.err
}

func (f *fakeClient) LibraryRoots(_ context.Context) ([]string, error) {
	if f.roots == nil {
		return nil, nil
	}
	return append([]string{}, f.roots...), nil
}

func (f *fakeClient) FindJob(_ context.Context, _ string) (stash.Job, error) {
	f.findCalls++
	if len(f.jobs) == 0 {
		return stash.Job{}, errors.New("job not found")
	}

	job := f.jobs[0]
	if len(f.jobs) > 1 {
		f.jobs = f.jobs[1:]
	}
	return job, nil
}

func (f *fakeClient) StopJob(_ context.Context, id string) error {
	f.stoppedJobID = id
	return f.stopErr
}
