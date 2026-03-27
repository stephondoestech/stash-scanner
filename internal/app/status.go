package app

import (
	"context"
	"time"

	"stash-scanner/internal/state"
	"stash-scanner/internal/version"
)

type RunSummary struct {
	Trigger         string    `json:"trigger"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	TrackedFiles    int       `json:"tracked_files"`
	DetectedTargets int       `json:"detected_targets"`
	PendingTargets  int       `json:"pending_targets"`
	PendingAfter    int       `json:"pending_after"`
	ScanTargets     int       `json:"scan_targets"`
	ScanAttempted   bool      `json:"scan_attempted"`
	ScanSucceeded   bool      `json:"scan_succeeded"`
	RetryAttempt    int       `json:"retry_attempt"`
	RetryDeferred   bool      `json:"retry_deferred"`
	StateSaved      bool      `json:"state_saved"`
	LastError       string    `json:"last_error,omitempty"`
}

type RunState struct {
	Trigger   string    `json:"trigger"`
	StartedAt time.Time `json:"started_at"`
	Phase     string    `json:"phase"`
	Detail    string    `json:"detail"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Status struct {
	Version             string            `json:"version"`
	Now                 time.Time         `json:"now"`
	Running             bool              `json:"running"`
	CurrentRun          RunState          `json:"current_run"`
	LastRun             RunSummary        `json:"last_run"`
	PendingScan         state.PendingScan `json:"pending_scan"`
	LastRunAt           time.Time         `json:"last_run_at"`
	LastSuccess         time.Time         `json:"last_success_at"`
	WatchRoots          []string          `json:"watch_roots"`
	WatchRootsFromStash bool              `json:"watch_roots_from_stash"`
	DryRun              bool              `json:"dry_run"`
}

func (r *Runner) Status(_ context.Context) (Status, error) {
	snapshot, err := r.store.Load()
	if err != nil {
		return Status{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return Status{
		Version:             version.Current(),
		Now:                 r.now(),
		Running:             r.running,
		CurrentRun:          r.currentRun,
		LastRun:             r.lastSummary,
		PendingScan:         snapshot.PendingScan,
		LastRunAt:           snapshot.LastRunAt,
		LastSuccess:         snapshot.LastSuccessAt,
		WatchRoots:          append([]string{}, r.cfg.WatchRoots...),
		WatchRootsFromStash: r.cfg.WatchRootsFromStash,
		DryRun:              r.cfg.DryRun,
	}, nil
}

func (r *Runner) beginRun(trigger string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return false
	}

	r.running = true
	now := r.now()
	r.currentRun = RunState{
		Trigger:   trigger,
		StartedAt: now,
		Phase:     "starting",
		Detail:    "Preparing scan run",
		UpdatedAt: now,
	}
	return true
}

func (r *Runner) finishRun(summary RunSummary, snapshot state.Snapshot) {
	r.mu.Lock()
	r.running = false
	r.currentRun = RunState{}
	r.lastSummary = summary
	r.mu.Unlock()

	r.logRunSummary(summary, snapshot)
}

func (r *Runner) logRunSummary(summary RunSummary, snapshot state.Snapshot) {
	if summary.FinishedAt.IsZero() {
		summary.FinishedAt = r.now()
	}

	r.logger.Printf(
		"run summary: trigger=%s tracked_files=%d detected_targets=%d pending_targets=%d pending_after=%d scan_targets=%d scan_attempted=%t scan_succeeded=%t retry_attempt=%d retry_deferred=%t state_saved=%t last_run_at=%s last_success_at=%s duration=%s",
		summary.Trigger,
		summary.TrackedFiles,
		summary.DetectedTargets,
		summary.PendingTargets,
		summary.PendingAfter,
		summary.ScanTargets,
		summary.ScanAttempted,
		summary.ScanSucceeded,
		summary.RetryAttempt,
		summary.RetryDeferred,
		summary.StateSaved,
		snapshot.LastRunAt.Format(time.RFC3339),
		snapshot.LastSuccessAt.Format(time.RFC3339),
		summary.FinishedAt.Sub(summary.StartedAt).Round(time.Millisecond),
	)
	if summary.LastError != "" {
		r.logger.Printf("run error: %s", summary.LastError)
	}
}

func (r *Runner) updateRunProgress(phase, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}

	r.currentRun.Phase = phase
	r.currentRun.Detail = detail
	r.currentRun.UpdatedAt = r.now()
}
