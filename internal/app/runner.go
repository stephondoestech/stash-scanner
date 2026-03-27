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
	pollEvery time.Duration

	mu          sync.RWMutex
	running     bool
	cancelRun   context.CancelFunc
	currentRun  RunState
	lastSummary RunSummary
}

type scanClient interface {
	TriggerScan(context.Context, []string) (string, error)
	LibraryRoots(context.Context) ([]string, error)
	FindJob(context.Context, string) (stash.Job, error)
	StopJob(context.Context, string) error
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
		pollEvery: 3 * time.Second,
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
	runCtx, cancel := context.WithCancel(context.Background())
	r.setRunCancel(cancel)

	go func() {
		defer cancel()
		if err := r.runCycleWithLock(runCtx, "manual"); err != nil {
			if errors.Is(err, context.Canceled) {
				r.logger.Printf("manual run stopped")
				return
			}
			r.logger.Printf("manual run failed: %v", err)
		}
	}()
	return nil
}

func (r *Runner) runCycle(ctx context.Context, trigger string) error {
	if !r.beginRun(trigger) {
		return ErrRunInProgress
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.setRunCancel(cancel)
	defer cancel()
	return r.runCycleWithLock(runCtx, trigger)
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

	r.updateRunProgress("resolving_roots", "Resolving watch roots")
	roots, err := r.watchRoots(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			summary.Stopped = true
			return err
		}
		summary.LastError = err.Error()
		return fmt.Errorf("resolve watch roots: %w", err)
	}

	r.updateRunProgress("scanning_files", fmt.Sprintf("Scanning %d watch roots", len(roots)))
	result, err := r.detector.Scan(ctx, roots, st.Paths)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			summary.Stopped = true
			return err
		}
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
		r.updateRunProgress("waiting_retry", "Retry deferred until backoff expires")
		st.PendingScan.Paths = uniqueSorted(append(st.PendingScan.Paths, result.Targets...))
	}

	if len(scanTargets) > 0 && !retryDeferred {
		summary.ScanAttempted = true
		r.updateRunProgress("triggering_scan", fmt.Sprintf("Requesting Stash scan for %d path targets", len(scanTargets)))
		jobID, err := r.client.TriggerScan(ctx, scanTargets)
		if err != nil {
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
		summary.StashTask.ID = jobID
		if jobID != "" {
			r.updateRunProgress("waiting_for_stash", fmt.Sprintf("Waiting for Stash task %s", jobID))
			task, waitErr := r.waitForJob(ctx, jobID)
			summary.StashTask = task
			if waitErr != nil {
				summary.LastError = waitErr.Error()
				if errors.Is(waitErr, context.Canceled) {
					summary.Stopped = true
				} else {
					st.PendingScan = r.nextPendingScan(scanTargets, st.PendingScan, waitErr, summary.StartedAt)
				}
				st.Paths = result.Current
				st.LastRunAt = result.FinishedAt
				finalSnapshot = st
				r.updateRunProgress("saving_state", "Saving scanner state")
				if saveErr := r.store.Save(st); saveErr != nil {
					summary.LastError = saveErr.Error()
					return fmt.Errorf("save failed state: %w", saveErr)
				}
				summary.StateSaved = true
				summary.PendingAfter = len(st.PendingScan.Paths)
				finalSnapshot = st
				return waitErr
			}
		}
		summary.ScanSucceeded = true
		st.PendingScan = state.PendingScan{}
	} else if len(scanTargets) == 0 {
		r.updateRunProgress("idle", "No changed scan targets found")
	}

	st.Paths = result.Current
	st.LastRunAt = result.FinishedAt
	if summary.ScanSucceeded || (!summary.ScanAttempted && len(st.PendingScan.Paths) == 0) {
		st.LastSuccessAt = result.FinishedAt
	}

	finalSnapshot = st
	r.updateRunProgress("saving_state", "Saving scanner state")
	if err := r.store.Save(st); err != nil {
		summary.LastError = err.Error()
		return fmt.Errorf("save state: %w", err)
	}

	summary.StateSaved = true
	summary.PendingAfter = len(st.PendingScan.Paths)
	finalSnapshot = st
	r.updateRunProgress("completed", "Scan run completed")
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

func (r *Runner) setRunCancel(cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	r.cancelRun = cancel
}
