package detect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"stash-scanner/internal/state"
)

type Detector struct {
	includePatterns []string
	excludePatterns []string
}

type Result struct {
	Current    map[string]state.PathState
	Targets    []string
	FinishedAt time.Time
}

func New(includePatterns, excludePatterns []string) *Detector {
	return &Detector{
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
	}
}

func (d *Detector) Scan(ctx context.Context, roots []string, previous map[string]state.PathState) (Result, error) {
	current := make(map[string]state.PathState)
	changedDirs := map[string]struct{}{}
	now := time.Now().UTC()

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !d.shouldInclude(path) {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			prev, exists := previous[path]
			current[path] = state.PathState{
				Path:        path,
				Size:        info.Size(),
				ModifiedAt:  info.ModTime().UTC(),
				FirstSeenAt: firstSeen(prev, exists, now),
				LastSeenAt:  now,
			}

			if !exists || prev.Size != info.Size() || !prev.ModifiedAt.Equal(info.ModTime().UTC()) {
				changedDirs[filepath.Dir(path)] = struct{}{}
			}

			return nil
		})
		if err != nil {
			return Result{}, fmt.Errorf("walk %s: %w", root, err)
		}
	}

	return Result{
		Current:    current,
		Targets:    coalesceTargets(changedDirs),
		FinishedAt: now,
	}, nil
}

func firstSeen(prev state.PathState, exists bool, now time.Time) time.Time {
	if exists && !prev.FirstSeenAt.IsZero() {
		return prev.FirstSeenAt
	}
	return now
}

func (d *Detector) shouldInclude(path string) bool {
	if matches(path, d.excludePatterns) {
		return false
	}
	if len(d.includePatterns) == 0 {
		return true
	}
	return matches(path, d.includePatterns)
}

func matches(path string, patterns []string) bool {
	base := filepath.Base(path)
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func coalesceTargets(changedDirs map[string]struct{}) []string {
	targets := make([]string, 0, len(changedDirs))
	for path := range changedDirs {
		targets = append(targets, path)
	}

	slices.Sort(targets)

	reduced := make([]string, 0, len(targets))
	for _, target := range targets {
		if len(reduced) == 0 || !isChildPath(reduced[len(reduced)-1], target) {
			reduced = append(reduced, target)
		}
	}

	return reduced
}

func isChildPath(parent, child string) bool {
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
