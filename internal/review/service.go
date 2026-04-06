package review

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"stash-scanner/internal/logging"
	"stash-scanner/internal/stash"
)

type stashClient interface {
	MissingPerformerScenes(context.Context) ([]stash.MediaItem, error)
	MissingPerformerGalleries(context.Context) ([]stash.MediaItem, error)
	Performers(context.Context) ([]stash.Performer, error)
	FetchImage(context.Context, string) (stash.ImageResult, error)
	AutoAssignGalleryPerformersFromScenePaths(context.Context) (int, error)
	AssignScenePerformers(context.Context, string, []string) error
	AssignGalleryPerformers(context.Context, string, []string) error
}

type Service struct {
	logger *log.Logger
	store  *Store
	client stashClient
	now    func() time.Time

	performers []stash.Performer
	mu         sync.RWMutex
	running    bool
	snapshot   Snapshot
}

func NewService(store *Store, client stashClient, logger *log.Logger) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if logger == nil {
		logger = log.Default()
	}

	snapshot, err := store.Load()
	if err != nil {
		return nil, err
	}

	return &Service{
		logger:   logger,
		store:    store,
		client:   client,
		now:      func() time.Time { return time.Now().UTC() },
		snapshot: snapshot,
	}, nil
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		Now:           s.now(),
		Running:       s.running,
		RefreshedAt:   s.snapshot.RefreshedAt,
		ItemCount:     s.snapshot.ItemCount,
		ActiveCount:   s.snapshot.ActiveCount,
		SkippedCount:  s.snapshot.SkippedCount,
		ResolvedCount: s.snapshot.ResolvedCount,
		ReviewCount:   s.snapshot.ReviewCount,
		EmptyCount:    s.snapshot.EmptyCount,
		LastError:     s.snapshot.LastError,
		Items:         append([]QueueItem{}, s.snapshot.Items...),
	}
}

func (s *Service) Refresh(ctx context.Context) error {
	if !s.beginRefresh() {
		return fmt.Errorf("refresh already in progress")
	}
	defer s.endRefresh()

	logging.Event(s.logger, "review_refresh_started")

	s.mu.RLock()
	previousItems := append([]QueueItem{}, s.snapshot.Items...)
	s.mu.RUnlock()

	scenes, err := s.client.MissingPerformerScenes(ctx)
	if err != nil {
		return s.fail(err)
	}
	autoAssigned, err := s.client.AutoAssignGalleryPerformersFromScenePaths(ctx)
	if err != nil {
		return s.fail(err)
	}
	galleries, err := s.client.MissingPerformerGalleries(ctx)
	if err != nil {
		return s.fail(err)
	}
	performers, err := s.client.Performers(ctx)
	if err != nil {
		return s.fail(err)
	}

	items := make([]QueueItem, 0, len(scenes)+len(galleries))
	for _, item := range scenes {
		items = append(items, scoreItem(item, SceneItem, performers))
	}
	for _, item := range galleries {
		items = append(items, scoreItem(item, GalleryItem, performers))
	}
	mergeReviewState(items, previousItems)

	sort.Slice(items, func(i, j int) bool {
		if items[i].ReviewState != items[j].ReviewState {
			return items[i].ReviewState < items[j].ReviewState
		}
		if items[i].Status != items[j].Status {
			return items[i].Status < items[j].Status
		}
		if items[i].BestScore != items[j].BestScore {
			return items[i].BestScore > items[j].BestScore
		}
		return items[i].Title < items[j].Title
	})

	snapshot := Snapshot{
		RefreshedAt: s.now(),
		ItemCount:   len(items),
		Items:       items,
	}
	for _, item := range items {
		if item.ReviewState == ReviewSkipped {
			snapshot.SkippedCount++
		} else if item.ReviewState == ReviewResolved {
			snapshot.ResolvedCount++
		} else {
			snapshot.ActiveCount++
		}
		if item.Status == "needs_review" {
			snapshot.ReviewCount++
			continue
		}
		snapshot.EmptyCount++
	}

	if err := s.store.Save(snapshot); err != nil {
		return s.fail(err)
	}

	s.mu.Lock()
	s.performers = append([]stash.Performer{}, performers...)
	s.snapshot = snapshot
	s.mu.Unlock()

	logging.Event(s.logger, "review_refresh_finished", "items", snapshot.ItemCount, "needs_review", snapshot.ReviewCount, "no_candidate", snapshot.EmptyCount, "auto_assigned_galleries", autoAssigned)
	return nil
}

func (s *Service) Run(ctx context.Context, interval time.Duration) error {
	if err := s.Refresh(ctx); err != nil {
		logging.Event(s.logger, "review_refresh_error", "error", err)
	}
	if interval <= 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil {
				logging.Event(s.logger, "review_refresh_error", "error", err)
			}
		}
	}
}

func (s *Service) beginRefresh() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Service) endRefresh() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *Service) fail(err error) error {
	s.mu.Lock()
	s.snapshot.LastError = err.Error()
	s.mu.Unlock()
	return err
}

func (s *Service) CandidateImage(ctx context.Context, itemID, performerID string) (stash.ImageResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.snapshot.Items {
		if item.ID != itemID {
			continue
		}
		for _, candidate := range item.Candidates {
			if candidate.PerformerID != performerID {
				continue
			}
			return s.client.FetchImage(ctx, candidate.ImageURL)
		}
		return stash.ImageResult{}, fmt.Errorf("candidate %q not found for item %q", performerID, itemID)
	}

	return stash.ImageResult{}, fmt.Errorf("item %q not found", itemID)
}

func (s *Service) SetReviewState(itemID string, state ReviewState) error {
	if itemID == "" {
		return fmt.Errorf("item id is required")
	}
	if state != ReviewPending && state != ReviewSkipped {
		return fmt.Errorf("unsupported review state %q", state)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	now := s.now()
	for i := range s.snapshot.Items {
		if s.snapshot.Items[i].ID != itemID {
			continue
		}
		s.snapshot.Items[i].ReviewState = state
		s.snapshot.Items[i].ReviewedAt = now
		if state != ReviewResolved {
			s.snapshot.Items[i].ResolvedAt = time.Time{}
			if state == ReviewPending {
				s.snapshot.Items[i].AssignedPerformerIDs = nil
			}
		}
		found = true
		break
	}
	if !found {
		return fmt.Errorf("item %q not found", itemID)
	}
	recountSnapshot(&s.snapshot)
	if err := s.store.Save(s.snapshot); err != nil {
		return err
	}
	return nil
}

func (s *Service) AssignCandidate(ctx context.Context, itemID, performerID string) error {
	if itemID == "" || performerID == "" {
		return fmt.Errorf("item id and performer id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	index := -1
	var item QueueItem
	for i := range s.snapshot.Items {
		if s.snapshot.Items[i].ID == itemID {
			index = i
			item = s.snapshot.Items[i]
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("item %q not found", itemID)
	}

	switch item.Type {
	case SceneItem:
		if err := s.client.AssignScenePerformers(ctx, item.ID, []string{performerID}); err != nil {
			return err
		}
	case GalleryItem:
		if err := s.client.AssignGalleryPerformers(ctx, item.ID, []string{performerID}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported item type %q", item.Type)
	}

	now := s.now()
	s.snapshot.Items[index].ReviewState = ReviewResolved
	s.snapshot.Items[index].ReviewedAt = now
	s.snapshot.Items[index].ResolvedAt = now
	s.snapshot.Items[index].AssignedPerformerIDs = []string{performerID}
	recountSnapshot(&s.snapshot)
	if err := s.store.Save(s.snapshot); err != nil {
		return err
	}
	return nil
}

func mergeReviewState(next []QueueItem, previous []QueueItem) {
	existing := make(map[string]QueueItem, len(previous))
	for _, item := range previous {
		existing[reviewKey(item.Type, item.ID)] = item
	}
	for i := range next {
		if prev, ok := existing[reviewKey(next[i].Type, next[i].ID)]; ok {
			next[i].ReviewState = prev.ReviewState
			next[i].ReviewedAt = prev.ReviewedAt
			next[i].ResolvedAt = prev.ResolvedAt
			next[i].AssignedPerformerIDs = append([]string{}, prev.AssignedPerformerIDs...)
		}
		if next[i].ReviewState == "" {
			next[i].ReviewState = ReviewPending
		}
	}
}

func reviewKey(itemType ItemType, id string) string {
	return string(itemType) + ":" + id
}

func recountSnapshot(snapshot *Snapshot) {
	snapshot.ItemCount = len(snapshot.Items)
	snapshot.ActiveCount = 0
	snapshot.SkippedCount = 0
	snapshot.ResolvedCount = 0
	snapshot.ReviewCount = 0
	snapshot.EmptyCount = 0
	for _, item := range snapshot.Items {
		if item.ReviewState == ReviewSkipped {
			snapshot.SkippedCount++
		} else if item.ReviewState == ReviewResolved {
			snapshot.ResolvedCount++
		} else {
			snapshot.ActiveCount++
		}
		if item.Status == "needs_review" {
			snapshot.ReviewCount++
		} else {
			snapshot.EmptyCount++
		}
	}
}
