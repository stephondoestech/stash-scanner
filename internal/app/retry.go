package app

import (
	"slices"
	"time"

	"stash-scanner/internal/logging"
	"stash-scanner/internal/state"
)

func (r *Runner) resolveScanTargets(detected []string, pending state.PendingScan, now time.Time) ([]string, int, bool) {
	targets := append([]string{}, detected...)
	attempt := 1
	retryDeferred := false
	if len(pending.Paths) == 0 {
		return uniqueSorted(targets), attempt, retryDeferred
	}

	if !pending.NextAttemptAt.IsZero() && pending.NextAttemptAt.After(now) {
		retryDeferred = true
		return nil, pending.AttemptCount + 1, retryDeferred
	}

	targets = append(targets, pending.Paths...)
	attempt = pending.AttemptCount + 1
	return uniqueSorted(targets), attempt, retryDeferred
}

func (r *Runner) nextPendingScan(paths []string, previous state.PendingScan, scanErr error, now time.Time) state.PendingScan {
	mergedPaths := uniqueSorted(append(append([]string{}, previous.Paths...), paths...))

	attemptCount := previous.AttemptCount + 1
	if len(previous.Paths) == 0 || (!previous.NextAttemptAt.IsZero() && previous.NextAttemptAt.After(now)) {
		attemptCount = 1
	}

	if attemptCount > r.cfg.Retry.MaxAttempts {
		attemptCount = r.cfg.Retry.MaxAttempts
	}

	firstFailedAt := previous.FirstFailedAt
	if firstFailedAt.IsZero() {
		firstFailedAt = now
	}

	next := state.PendingScan{
		Paths:         mergedPaths,
		AttemptCount:  attemptCount,
		LastError:     scanErr.Error(),
		FirstFailedAt: firstFailedAt,
		LastFailedAt:  now,
		NextAttemptAt: now.Add(r.backoffForAttempt(attemptCount)),
	}
	logging.DebugEvent(
		r.logger,
		"retry_scheduled",
		"attempt", next.AttemptCount,
		"next_attempt_at", next.NextAttemptAt.Format(time.RFC3339),
		"pending_paths", next.Paths,
		"error", scanErr,
	)
	return next
}

func (r *Runner) backoffForAttempt(attempt int) time.Duration {
	backoff := r.cfg.Retry.InitialBackoff.Duration
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= r.cfg.Retry.MaxBackoff.Duration {
			return r.cfg.Retry.MaxBackoff.Duration
		}
	}
	return backoff
}

func uniqueSorted(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	unique := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	slices.Sort(unique)
	return unique
}
