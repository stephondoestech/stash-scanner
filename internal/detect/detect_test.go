package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"stash-scanner/internal/state"
)

func TestScanDetectsNewFilesAndCoalescesTargets(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "library", "scene")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fileA := filepath.Join(subdir, "a.mp4")
	fileB := filepath.Join(subdir, "b.mp4")
	writeTestFile(t, fileA, "a")
	writeTestFile(t, fileB, "b")

	detector := New([]string{"*.mp4"}, []string{".cache"})
	result, err := detector.Scan(context.Background(), []string{root}, map[string]state.PathState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(result.Current) != 2 {
		t.Fatalf("expected 2 tracked files, got %d", len(result.Current))
	}

	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 coalesced target, got %d", len(result.Targets))
	}

	if got, want := result.Targets[0], subdir; got != want {
		t.Fatalf("target mismatch: got %q want %q", got, want)
	}
}

func TestScanIgnoresUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "scene.mp4")
	writeTestFile(t, filePath, "stable")

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	now := time.Now().UTC()
	previous := map[string]state.PathState{
		filePath: {
			Path:        filePath,
			Size:        info.Size(),
			ModifiedAt:  info.ModTime().UTC(),
			FirstSeenAt: now.Add(-time.Hour),
			LastSeenAt:  now.Add(-time.Minute),
		},
	}

	detector := New([]string{"*.mp4"}, nil)
	result, err := detector.Scan(context.Background(), []string{root}, previous)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(result.Targets) != 0 {
		t.Fatalf("expected no changed targets, got %d", len(result.Targets))
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
