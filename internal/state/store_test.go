package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingStateReturnsEmptySnapshot(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing", "state.json"))

	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(snapshot.Paths) != 0 {
		t.Fatalf("expected empty path state, got %d entries", len(snapshot.Paths))
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{
			"/media/example.mp4": {
				Path:        "/media/example.mp4",
				Size:        123,
				ModifiedAt:  now,
				FirstSeenAt: now.Add(-time.Hour),
				LastSeenAt:  now,
			},
		},
		LastRunAt:     now,
		LastSuccessAt: now,
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	output, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := len(output.Paths), 1; got != want {
		t.Fatalf("path count mismatch: got %d want %d", got, want)
	}

	if got, want := output.Paths["/media/example.mp4"].Size, int64(123); got != want {
		t.Fatalf("size mismatch: got %d want %d", got, want)
	}

	if got, want := output.PendingScan.AttemptCount, 0; got != want {
		t.Fatalf("pending attempt count mismatch: got %d want %d", got, want)
	}

}

func TestSaveAndLoadPendingScanRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{},
		PendingScan: PendingScan{
			Paths:         []string{"/media/a", "/media/b"},
			AttemptCount:  2,
			LastError:     "temporary failure",
			FirstFailedAt: now.Add(-time.Minute),
			LastFailedAt:  now,
			NextAttemptAt: now.Add(time.Minute),
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	output, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := len(output.PendingScan.Paths), 2; got != want {
		t.Fatalf("pending path count mismatch: got %d want %d", got, want)
	}

	if got, want := output.PendingScan.AttemptCount, 2; got != want {
		t.Fatalf("pending attempt count mismatch: got %d want %d", got, want)
	}
}

func TestSaveAndLoadPendingDebounceRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{},
		PendingDebounce: PendingDebounce{
			Paths:          []string{"/media/a"},
			LastDetectedAt: now,
			ReadyAt:        now.Add(30 * time.Second),
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	output, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := len(output.PendingDebounce.Paths), 1; got != want {
		t.Fatalf("pending debounce path count mismatch: got %d want %d", got, want)
	}

	if got, want := output.PendingDebounce.ReadyAt, input.PendingDebounce.ReadyAt; !got.Equal(want) {
		t.Fatalf("pending debounce ready time mismatch: got %s want %s", got, want)
	}
}

func TestLoadMetadataReadsSidecarWithoutPaths(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{
			"/media/example.mp4": {
				Path:        "/media/example.mp4",
				Size:        123,
				ModifiedAt:  now,
				FirstSeenAt: now.Add(-time.Hour),
				LastSeenAt:  now,
			},
		},
		PendingScan: PendingScan{
			Paths:        []string{"/media/example"},
			AttemptCount: 1,
		},
		PendingDebounce: PendingDebounce{
			Paths: []string{"/media/debounce"},
		},
		LastRunAt:     now,
		LastSuccessAt: now,
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	metadata, err := store.LoadMetadata()
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	if got, want := len(metadata.PendingScan.Paths), 1; got != want {
		t.Fatalf("pending scan count mismatch: got %d want %d", got, want)
	}

	if got, want := len(metadata.PendingDebounce.Paths), 1; got != want {
		t.Fatalf("pending debounce count mismatch: got %d want %d", got, want)
	}

	if got, want := metadata.LastSuccessAt, now; !got.Equal(want) {
		t.Fatalf("last success mismatch: got %s want %s", got, want)
	}
}

func TestLoadMetadataFallsBackToFullStateWhenSidecarIsMissing(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{},
		PendingScan: PendingScan{
			Paths:        []string{"/media/example"},
			AttemptCount: 2,
		},
		LastRunAt: now,
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := os.Remove(store.metadataPath()); err != nil {
		t.Fatalf("remove metadata sidecar: %v", err)
	}

	metadata, err := store.LoadMetadata()
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	if got, want := metadata.PendingScan.AttemptCount, 2; got != want {
		t.Fatalf("pending attempt count mismatch: got %d want %d", got, want)
	}
}

func TestRequeuePathsRemovesTrackedEntriesBeneathRequestedPath(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{
			"/media/keep.mp4": {
				Path:        "/media/keep.mp4",
				ModifiedAt:  now,
				FirstSeenAt: now,
				LastSeenAt:  now,
			},
			"/media/performer/scene-1.mp4": {
				Path:        "/media/performer/scene-1.mp4",
				ModifiedAt:  now,
				FirstSeenAt: now,
				LastSeenAt:  now,
			},
			"/media/performer/subdir/scene-2.mp4": {
				Path:        "/media/performer/subdir/scene-2.mp4",
				ModifiedAt:  now,
				FirstSeenAt: now,
				LastSeenAt:  now,
			},
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	removed, err := store.RequeuePaths([]string{"/media/performer"})
	if err != nil {
		t.Fatalf("RequeuePaths: %v", err)
	}

	if got, want := removed, 2; got != want {
		t.Fatalf("removed count mismatch: got %d want %d", got, want)
	}

	output, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := len(output.Paths), 1; got != want {
		t.Fatalf("path count mismatch: got %d want %d", got, want)
	}

	if _, ok := output.Paths["/media/keep.mp4"]; !ok {
		t.Fatal("expected unrelated path to remain in state")
	}
}

func TestRequeuePathsClearsOverlappingPendingQueues(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "data", "state.json"))
	now := time.Now().UTC().Round(time.Second)
	input := Snapshot{
		Paths: map[string]PathState{
			"/media/performer/scene.mp4": {
				Path:        "/media/performer/scene.mp4",
				ModifiedAt:  now,
				FirstSeenAt: now,
				LastSeenAt:  now,
			},
		},
		PendingScan: PendingScan{
			Paths:         []string{"/media/performer", "/media/keep"},
			AttemptCount:  2,
			LastError:     "temporary failure",
			FirstFailedAt: now.Add(-time.Minute),
			LastFailedAt:  now,
			NextAttemptAt: now.Add(time.Minute),
		},
		PendingDebounce: PendingDebounce{
			Paths:          []string{"/media/performer/subdir", "/media/other"},
			LastDetectedAt: now,
			ReadyAt:        now.Add(30 * time.Second),
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := store.RequeuePaths([]string{"/media/performer"}); err != nil {
		t.Fatalf("RequeuePaths: %v", err)
	}

	output, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := len(output.PendingScan.Paths), 1; got != want {
		t.Fatalf("pending scan count mismatch: got %d want %d", got, want)
	}

	if got, want := output.PendingScan.Paths[0], "/media/keep"; got != want {
		t.Fatalf("pending scan path mismatch: got %q want %q", got, want)
	}

	if got, want := len(output.PendingDebounce.Paths), 1; got != want {
		t.Fatalf("pending debounce count mismatch: got %d want %d", got, want)
	}

	if got, want := output.PendingDebounce.Paths[0], "/media/other"; got != want {
		t.Fatalf("pending debounce path mismatch: got %q want %q", got, want)
	}
}
