package app

import (
	"context"
	"errors"
	"fmt"

	"stash-scanner/internal/logging"
)

var ErrNoRunInProgress = errors.New("no active scan run")
var ErrNoPendingDebounce = errors.New("no pending debounce paths")

func (r *Runner) StopActiveRun(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return ErrNoRunInProgress
	}

	cancel := r.cancelRun
	taskID := r.currentRun.StashTask.ID
	r.currentRun.Phase = "stopping"
	r.currentRun.Detail = "Stopping active scan run"
	r.currentRun.UpdatedAt = r.now()
	r.mu.Unlock()

	logging.Event(r.logger, "stop_requested", "stash_task_id", taskID)

	if cancel != nil {
		cancel()
	}
	if taskID == "" {
		return nil
	}
	if err := r.client.StopJob(ctx, taskID); err != nil {
		return fmt.Errorf("stop stash job %s: %w", taskID, err)
	}

	return nil
}

func (r *Runner) FlushPendingDebounce() error {
	r.requestDebounceFlush()

	st, err := r.store.Load()
	if err != nil {
		return err
	}

	if len(st.PendingDebounce.Paths) == 0 {
		r.mu.RLock()
		running := r.running
		r.mu.RUnlock()
		if running {
			return nil
		}
		return ErrNoPendingDebounce
	}

	st.PendingDebounce.ReadyAt = r.now()
	if err := r.store.Save(st); err != nil {
		return err
	}

	r.mu.RLock()
	running := r.running
	r.mu.RUnlock()
	if running {
		return nil
	}

	if err := r.StartManualRun(); err != nil && !errors.Is(err, ErrRunInProgress) {
		return err
	}
	return nil
}
