package app

import (
	"context"
	"fmt"
	"time"

	"stash-scanner/internal/stash"
)

func (r *Runner) waitForJob(ctx context.Context, jobID string) (StashTaskStatus, error) {
	for {
		task, err := r.client.FindJob(ctx, jobID)
		if err != nil {
			return StashTaskStatus{ID: jobID}, fmt.Errorf("find stash job %s: %w", jobID, err)
		}

		status := toTaskStatus(task)
		r.updateRunTask(status)
		r.updateRunProgress("waiting_for_stash", formatTaskDetail(status))

		switch status.Status {
		case "FINISHED":
			return status, nil
		case "FAILED":
			return status, fmt.Errorf("stash job failed: %s", status.Error)
		case "CANCELLED":
			return status, context.Canceled
		}

		timer := time.NewTimer(r.pollEvery)
		select {
		case <-ctx.Done():
			timer.Stop()
			return status, ctx.Err()
		case <-timer.C:
		}
	}
}

func toTaskStatus(job stash.Job) StashTaskStatus {
	return StashTaskStatus{
		ID:          job.ID,
		Status:      job.Status,
		Description: job.Description,
		Progress:    job.Progress,
		AddedAt:     job.AddTime,
		StartedAt:   job.StartTime,
		EndedAt:     job.EndTime,
		Error:       job.Error,
	}
}

func formatTaskDetail(task StashTaskStatus) string {
	if task.Description == "" {
		return "Waiting for Stash task"
	}
	if task.Progress <= 0 {
		return fmt.Sprintf("%s (%s)", task.Description, task.Status)
	}
	return fmt.Sprintf("%s (%.0f%%, %s)", task.Description, task.Progress*100, task.Status)
}
