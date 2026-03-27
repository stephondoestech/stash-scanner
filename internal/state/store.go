package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type PathState struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modified_at"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

type Snapshot struct {
	Paths           map[string]PathState `json:"paths"`
	PendingScan     PendingScan          `json:"pending_scan"`
	PendingDebounce PendingDebounce      `json:"pending_debounce"`
	LastRunAt       time.Time            `json:"last_run_at"`
	LastSuccessAt   time.Time            `json:"last_success_at"`
}

type PendingScan struct {
	Paths         []string  `json:"paths"`
	AttemptCount  int       `json:"attempt_count"`
	LastError     string    `json:"last_error,omitempty"`
	FirstFailedAt time.Time `json:"first_failed_at"`
	LastFailedAt  time.Time `json:"last_failed_at"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
}

type PendingDebounce struct {
	Paths          []string  `json:"paths"`
	LastDetectedAt time.Time `json:"last_detected_at"`
	ReadyAt        time.Time `json:"ready_at"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{Paths: map[string]PathState{}}, nil
		}
		return Snapshot{}, fmt.Errorf("read state file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode state file: %w", err)
	}

	if snapshot.Paths == nil {
		snapshot.Paths = map[string]PathState{}
	}

	return snapshot, nil
}

func (s *Store) Save(snapshot Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}
