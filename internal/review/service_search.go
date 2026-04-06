package review

import (
	"fmt"
	"strings"

	"stash-scanner/internal/stash"
)

func (s *Service) SearchPerformers(query string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	s.mu.RLock()
	performers := append([]stash.Performer{}, s.performers...)
	s.mu.RUnlock()

	if len(performers) == 0 {
		return nil, fmt.Errorf("performers not loaded; refresh queue first")
	}

	return searchPerformers(query, performers), nil
}
