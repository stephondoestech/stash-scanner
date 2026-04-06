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
}

type Service struct {
	logger *log.Logger
	store  *Store
	client stashClient
	now    func() time.Time

	mu       sync.RWMutex
	running  bool
	snapshot Snapshot
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
		Now:         s.now(),
		Running:     s.running,
		RefreshedAt: s.snapshot.RefreshedAt,
		ItemCount:   s.snapshot.ItemCount,
		ReviewCount: s.snapshot.ReviewCount,
		EmptyCount:  s.snapshot.EmptyCount,
		LastError:   s.snapshot.LastError,
		Items:       append([]QueueItem{}, s.snapshot.Items...),
	}
}

func (s *Service) Refresh(ctx context.Context) error {
	if !s.beginRefresh() {
		return fmt.Errorf("refresh already in progress")
	}
	defer s.endRefresh()

	logging.Event(s.logger, "review_refresh_started")

	scenes, err := s.client.MissingPerformerScenes(ctx)
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

	sort.Slice(items, func(i, j int) bool {
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
	s.snapshot = snapshot
	s.mu.Unlock()

	logging.Event(s.logger, "review_refresh_finished", "items", snapshot.ItemCount, "needs_review", snapshot.ReviewCount, "no_candidate", snapshot.EmptyCount)
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
