package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"stash-scanner/internal/config"
	"stash-scanner/internal/detect"
	"stash-scanner/internal/scheduler"
	"stash-scanner/internal/stash"
	"stash-scanner/internal/state"
)

var ErrRunInProgress = errors.New("scan run already in progress")

type Runner struct {
	cfg       config.Config
	logger    *log.Logger
	detector  *detect.Detector
	store     *state.Store
	client    scanClient
	scheduler *scheduler.Scheduler
	now       func() time.Time

	mu          sync.RWMutex
	running     bool
	currentRun  RunState
	lastSummary RunSummary
}

type scanClient interface {
	TriggerScan(context.Context, []string) error
	LibraryRoots(context.Context) ([]string, error)
}

func NewRunner(cfg config.Config, logger *log.Logger) (*Runner, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &Runner{
		cfg:       cfg,
		logger:    logger,
		detector:  detect.New(cfg.IncludePatterns, cfg.ExcludePatterns),
		store:     state.NewStore(cfg.StatePath),
		client:    stash.NewClient(cfg.StashURL, cfg.APIKey, cfg.DryRun),
		scheduler: scheduler.New(cfg.Schedule.Interval.Duration, cfg.Schedule.DailyTime),
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	r.logger.Printf("service started with %d watch roots", len(r.cfg.WatchRoots))
	return r.scheduler.Run(ctx, func(runCtx context.Context) error {
		err := r.runCycle(runCtx, "scheduled")
		if errors.Is(err, ErrRunInProgress) {
			r.logger.Printf("scheduled run skipped: %v", err)
			return nil
		}
		return err
	})
}

func (r *Runner) RunOnce(ctx context.Context) error {
	return r.runCycle(ctx, "once")
}

func (r *Runner) StartManualRun() error {
	if !r.beginRun("manual") {
		return ErrRunInProgress
	}

	go func() {
		if err := r.runCycleWithLock(context.Background(), "manual"); err != nil {
			r.logger.Printf("manual run failed: %v", err)
		}
	}()
	return nil
}

func (r *Runner) runCycle(ctx context.Context, trigger string) error {
	if !r.beginRun(trigger) {
		return ErrRunInProgress
	}
	return r.runCycleWithLock(ctx, trigger)
}

func (r *Runner) runCycleWithLock(ctx context.Context, trigger string) error {
	summary := RunSummary{Trigger: trigger, StartedAt: r.now()}
	var finalSnapshot state.Snapshot

	defer func() {
		summary.FinishedAt = r.now()
		r.finishRun(summary, finalSnapshot)
	}()

	st, err := r.store.Load()
	if err != nil {
		summary.LastError = err.Error()
		return fmt.Errorf("load state: %w", err)
	}
	finalSnapshot = st

	roots, err := r.watchRoots(ctx)
	if err != nil {
		summary.LastError = err.Error()
		return fmt.Errorf("resolve watch roots: %w", err)
	}

	result, err := r.detector.Scan(roots, st.Paths)
	if err != nil {
		summary.LastError = err.Error()
		return fmt.Errorf("detect changes: %w", err)
	}
	summary.TrackedFiles = len(result.Current)
	summary.DetectedTargets = len(result.Targets)
	summary.PendingTargets = len(st.PendingScan.Paths)

	scanTargets, retryAttempt, retryDeferred := r.resolveScanTargets(result.Targets, st.PendingScan, summary.StartedAt)
	summary.ScanTargets = len(scanTargets)
	summary.RetryAttempt = retryAttempt
	summary.RetryDeferred = retryDeferred

	if retryDeferred {
		st.PendingScan.Paths = uniqueSorted(append(st.PendingScan.Paths, result.Targets...))
	}

	if len(scanTargets) > 0 && !retryDeferred {
		summary.ScanAttempted = true
		if err := r.client.TriggerScan(ctx, scanTargets); err != nil {
			summary.LastError = err.Error()
			st.PendingScan = r.nextPendingScan(scanTargets, st.PendingScan, err, summary.StartedAt)
			st.Paths = result.Current
			st.LastRunAt = result.FinishedAt
			finalSnapshot = st
			if saveErr := r.store.Save(st); saveErr != nil {
				summary.LastError = saveErr.Error()
				return fmt.Errorf("save failed state: %w", saveErr)
			}
			summary.StateSaved = true
			summary.PendingAfter = len(st.PendingScan.Paths)
			finalSnapshot = st
			return fmt.Errorf("trigger scan: %w", err)
		}
		summary.ScanSucceeded = true
		st.PendingScan = state.PendingScan{}
	}

	st.Paths = result.Current
	st.LastRunAt = result.FinishedAt
	if summary.ScanSucceeded || (!summary.ScanAttempted && len(st.PendingScan.Paths) == 0) {
		st.LastSuccessAt = result.FinishedAt
	}

	finalSnapshot = st
	if err := r.store.Save(st); err != nil {
		summary.LastError = err.Error()
		return fmt.Errorf("save state: %w", err)
	}

	summary.StateSaved = true
	summary.PendingAfter = len(st.PendingScan.Paths)
	finalSnapshot = st
	return nil
}

func (r *Runner) watchRoots(ctx context.Context) ([]string, error) {
	if !r.cfg.WatchRootsFromStash {
		return append([]string{}, r.cfg.WatchRoots...), nil
	}

	roots, err := r.client.LibraryRoots(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cfg.WatchRoots = append([]string{}, roots...)
	r.mu.Unlock()
	return roots, nil
}
