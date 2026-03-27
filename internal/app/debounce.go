package app

import (
	"time"

	"stash-scanner/internal/state"
)

func (r *Runner) nextPendingDebounce(detected []string, previous state.PendingDebounce, now time.Time) state.PendingDebounce {
	if len(detected) == 0 {
		return previous
	}

	next := state.PendingDebounce{
		Paths:          uniqueSorted(append(append([]string{}, previous.Paths...), detected...)),
		LastDetectedAt: now,
		ReadyAt:        now.Add(r.cfg.DebounceWindow.Duration),
	}
	if r.cfg.DebounceWindow.Duration <= 0 {
		next.ReadyAt = now
	}
	return next
}

func (r *Runner) resolveDebounceTargets(pending state.PendingDebounce, now time.Time) ([]string, bool) {
	if len(pending.Paths) == 0 {
		return nil, false
	}
	if pending.ReadyAt.After(now) {
		return nil, true
	}
	return append([]string{}, pending.Paths...), false
}

func clearPendingDebounce(pending state.PendingDebounce, scanned []string) state.PendingDebounce {
	if len(pending.Paths) == 0 || len(scanned) == 0 {
		return pending
	}

	remaining := make([]string, 0, len(pending.Paths))
	remove := make(map[string]struct{}, len(scanned))
	for _, path := range scanned {
		remove[path] = struct{}{}
	}
	for _, path := range pending.Paths {
		if _, ok := remove[path]; ok {
			continue
		}
		remaining = append(remaining, path)
	}
	if len(remaining) == 0 {
		return state.PendingDebounce{}
	}

	pending.Paths = remaining
	return pending
}
