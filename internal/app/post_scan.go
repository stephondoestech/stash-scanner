package app

import (
	"context"
	"fmt"

	"stash-scanner/internal/logging"
	"stash-scanner/internal/stash"
)

func (r *Runner) runPostScanTasks(ctx context.Context, paths []string) ([]string, StashTaskStatus, error) {
	if len(r.cfg.PostScan.Tasks) == 0 {
		return nil, StashTaskStatus{}, nil
	}

	completed := make([]string, 0, len(r.cfg.PostScan.Tasks))
	var lastTask StashTaskStatus
	for _, taskName := range r.cfg.PostScan.Tasks {
		task := stash.PostScanTask(taskName)
		r.updateRunProgress("post_scan_task", fmt.Sprintf("Running post-scan task %s", taskName))
		logging.Event(r.logger, "post_scan_task_started", "task", taskName, "targets", len(paths))
		if task == stash.PostScanIdentify {
			identifySources, describeErr := r.client.DescribeIdentifySources(ctx, r.cfg.PostScan)
			if describeErr != nil {
				return completed, lastTask, fmt.Errorf("identify sources: %w", describeErr)
			}
			r.updateIdentifySources(identifySources)
			lastTask.Description = "Post-scan task: identify"
			logging.Event(r.logger, "post_scan_identify_sources", "sources", identifySources)
		}

		jobID, err := r.client.TriggerPostScanTask(ctx, task, paths, r.cfg.PostScan)
		if err != nil {
			return completed, lastTask, fmt.Errorf("post-scan task %s: %w", taskName, err)
		}

		lastTask = StashTaskStatus{ID: jobID, Description: "Post-scan task: " + taskName}
		r.updateRunTask(lastTask)
		if jobID != "" {
			taskStatus, waitErr := r.waitForJob(ctx, jobID)
			lastTask = taskStatus
			if waitErr != nil {
				return completed, lastTask, fmt.Errorf("wait for post-scan task %s: %w", taskName, waitErr)
			}
		}

		completed = append(completed, taskName)
		if task == stash.PostScanIdentify {
			lastTask.Description = "Post-scan task: identify"
		}
		logging.Event(r.logger, "post_scan_task_finished", "task", taskName, "job_id", jobID)
	}

	return completed, lastTask, nil
}
