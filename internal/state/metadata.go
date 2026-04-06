package state

import (
	"encoding/json"
	"fmt"
	"os"
)

func (s *Store) LoadMetadata() (SnapshotMetadata, error) {
	data, err := os.ReadFile(s.metadataPath())
	if err == nil {
		var metadata SnapshotMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return SnapshotMetadata{}, fmt.Errorf("decode metadata file: %w", err)
		}
		return metadata, nil
	}
	if !os.IsNotExist(err) {
		return SnapshotMetadata{}, fmt.Errorf("read metadata file: %w", err)
	}

	snapshot, err := s.Load()
	if err != nil {
		return SnapshotMetadata{}, err
	}
	return metadataFromSnapshot(snapshot), nil
}
