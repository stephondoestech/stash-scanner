package state

import (
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
