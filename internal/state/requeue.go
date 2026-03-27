package state

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (s *Store) RequeuePaths(paths []string) (int, error) {
	requested := normalizePaths(paths)
	if len(requested) == 0 {
		return 0, fmt.Errorf("at least one path is required")
	}

	snapshot, err := s.Load()
	if err != nil {
		return 0, err
	}

	removed := 0
	for trackedPath := range snapshot.Paths {
		if matchesAnyRequestedPath(trackedPath, requested) {
			delete(snapshot.Paths, trackedPath)
			removed++
		}
	}

	snapshot.PendingScan.Paths = filterOverlappingPaths(snapshot.PendingScan.Paths, requested)
	if len(snapshot.PendingScan.Paths) == 0 {
		snapshot.PendingScan = PendingScan{}
	}

	snapshot.PendingDebounce.Paths = filterOverlappingPaths(snapshot.PendingDebounce.Paths, requested)
	if len(snapshot.PendingDebounce.Paths) == 0 {
		snapshot.PendingDebounce = PendingDebounce{}
	}

	if err := s.Save(snapshot); err != nil {
		return 0, err
	}

	return removed, nil
}

func normalizePaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}

		cleaned := filepath.Clean(trimmed)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	return normalized
}

func matchesAnyRequestedPath(path string, requested []string) bool {
	for _, candidate := range requested {
		if isSameOrChildPath(candidate, path) {
			return true
		}
	}
	return false
}

func filterOverlappingPaths(paths, requested []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if overlapsRequestedPath(path, requested) {
			continue
		}
		filtered = append(filtered, path)
	}
	return filtered
}

func overlapsRequestedPath(path string, requested []string) bool {
	for _, candidate := range requested {
		if isSameOrChildPath(candidate, path) || isSameOrChildPath(path, candidate) {
			return true
		}
	}
	return false
}

func isSameOrChildPath(parent, child string) bool {
	if parent == child {
		return true
	}

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
