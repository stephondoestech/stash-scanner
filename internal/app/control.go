package app

import (
	"context"
	"errors"
	"fmt"
)

var ErrNoRunInProgress = errors.New("no active scan run")

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
