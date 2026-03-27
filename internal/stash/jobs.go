package stash

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stash-scanner/internal/logging"
)

type Job struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Progress    float64   `json:"progress"`
	AddTime     time.Time `json:"addTime"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Error       string    `json:"error"`
}

func (c *Client) FindJob(ctx context.Context, id string) (Job, error) {
	if strings.TrimSpace(id) == "" {
		return Job{}, fmt.Errorf("job id is required")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return Job{}, err
	}

	response, err := c.executeQuery(ctx, endpoint, buildFindJobQuery(id))
	if err != nil {
		return Job{}, err
	}
	if response.Data.FindJob == nil {
		return Job{}, fmt.Errorf("stash job %q was not found", id)
	}

	return *response.Data.FindJob, nil
}

func (c *Client) StopJob(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("job id is required")
	}

	if c.dryRun {
		logging.Event(c.logger, "stash_stop_dry_run", "job_id", id)
		return nil
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return err
	}

	response, err := c.executeQuery(ctx, endpoint, buildStopJobMutation(id))
	if err != nil {
		return err
	}
	if !response.Data.StopJob {
		return fmt.Errorf("stash did not stop job %s", id)
	}

	logging.Event(c.logger, "stash_stop_requested", "job_id", id)
	return nil
}

func buildFindJobQuery(id string) string {
	return "query { findJob(input: { id: " + quoteString(id) + " }) { id status description progress addTime startTime endTime error } }"
}

func buildStopJobMutation(id string) string {
	return "mutation { stopJob(job_id: " + quoteString(id) + ") }"
}

func quoteString(value string) string {
	quoted, _ := json.Marshal(value)
	return string(quoted)
}
