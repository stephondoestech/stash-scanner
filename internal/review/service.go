package review

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"stash-scanner/internal/logging"
	"stash-scanner/internal/stash"
)

type stashClient interface {
	SceneItems(context.Context) ([]stash.MediaItem, error)
	GalleryItems(context.Context) ([]stash.MediaItem, error)
	Performers(context.Context) ([]stash.Performer, error)
	FetchImage(context.Context, string) (stash.ImageResult, error)
	AutoAssignGalleryPerformersFromScenePaths(context.Context) (int, error)
	AssignScenePerformers(context.Context, string, []string) error
	AssignGalleryPerformers(context.Context, string, []string) error
	RepairPerformer(context.Context, string) error
}

type Service struct {
	logger *log.Logger
	store  *Store
	client stashClient
	now    func() time.Time
	match  matchConfig

	performers []stash.Performer
	mu         sync.RWMutex
	running    bool
	snapshot   Snapshot
}

const maxAuditEntries = 50

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
		match:    matchConfigFromSettings(snapshot.Settings),
		snapshot: snapshot,
	}, nil
}

func (s *Service) SetMatchConfig(cfg matchConfig) {
	if cfg.MinCandidateScore < 1 {
		cfg.MinCandidateScore = defaultMatchConfig().MinCandidateScore
	}
	if cfg.MinCandidateLead < 0 {
		cfg.MinCandidateLead = defaultMatchConfig().MinCandidateLead
	}
	s.mu.Lock()
	s.match = cfg
	s.snapshot.Settings = cfg.settings()
	s.mu.Unlock()
}

func (s *Service) MatchConfig() matchConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.match
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		Now:             s.now(),
		Running:         s.running,
		RefreshedAt:     s.snapshot.RefreshedAt,
		ItemCount:       s.snapshot.ItemCount,
		ActiveCount:     s.snapshot.ActiveCount,
		SkippedCount:    s.snapshot.SkippedCount,
		ResolvedCount:   s.snapshot.ResolvedCount,
		ReviewCount:     s.snapshot.ReviewCount,
		RepairCount:     s.snapshot.RepairCount,
		SuppressedCount: s.snapshot.SuppressedCount,
		EmptyCount:      s.snapshot.EmptyCount,
		MatchMinScore:   s.match.MinCandidateScore,
		MatchMinLead:    s.match.MinCandidateLead,
		AuditTrail:      append([]AuditEntry{}, s.snapshot.AuditTrail...),
		LastError:       s.snapshot.LastError,
		Items:           append([]QueueItem{}, s.snapshot.Items...),
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

	performers, err := s.client.Performers(ctx)
	if err != nil {
		return s.fail(err)
	}
	completePerformers := make([]stash.Performer, 0, len(performers))
	performerByID := make(map[string]stash.Performer, len(performers))
	for _, performer := range performers {
		performerByID[performer.ID] = performer
		if !isIncompletePerformer(performer) {
			completePerformers = append(completePerformers, performer)
		}
	}
	scenes, err := s.client.SceneItems(ctx)
	if err != nil {
		return s.fail(err)
	}
	autoAssigned, err := s.client.AutoAssignGalleryPerformersFromScenePaths(ctx)
	if err != nil {
		return s.fail(err)
	}
	galleries, err := s.client.GalleryItems(ctx)
	if err != nil {
		return s.fail(err)
	}

	s.mu.RLock()
	matchCfg := s.match
	s.mu.RUnlock()
	items := make([]QueueItem, 0, len(scenes)+len(galleries))
	for _, item := range scenes {
		if queueItem, ok := buildQueueItem(item, SceneItem, performerByID, completePerformers, matchCfg); ok {
			items = append(items, queueItem)
		}
	}
	for _, item := range galleries {
		if queueItem, ok := buildQueueItem(item, GalleryItem, performerByID, completePerformers, matchCfg); ok {
			items = append(items, queueItem)
		}
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
		Settings:    matchCfg.settings(),
		AuditTrail:  append([]AuditEntry{}, s.snapshot.AuditTrail...),
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
		if item.Status == "repair_needed" {
			snapshot.RepairCount++
			continue
		}
		if item.Status == "suppressed" {
			snapshot.SuppressedCount++
			continue
		}
		snapshot.EmptyCount++
	}
	appendAuditEntry(&snapshot, AuditEntry{
		At:            s.now(),
		Action:        "refresh",
		Detail:        fmt.Sprintf("review queue refreshed with %d items", snapshot.ItemCount),
		MatchMinScore: matchCfg.MinCandidateScore,
		MatchMinLead:  matchCfg.MinCandidateLead,
	})

	if err := s.store.Save(snapshot); err != nil {
		return s.fail(err)
	}

	s.mu.Lock()
	s.performers = append([]stash.Performer{}, performers...)
	s.snapshot = snapshot
	s.mu.Unlock()

	logging.Event(s.logger, "review_refresh_finished", "items", snapshot.ItemCount, "needs_review", snapshot.ReviewCount, "repair_needed", snapshot.RepairCount, "suppressed", snapshot.SuppressedCount, "no_candidate", snapshot.EmptyCount, "auto_assigned_galleries", autoAssigned)
	return nil
}

func (s *Service) UpdateMatchConfig(ctx context.Context, cfg matchConfig) error {
	s.SetMatchConfig(cfg)
	s.mu.Lock()
	s.appendAuditLocked("settings_updated", fmt.Sprintf("reviewer thresholds set to min score %d and min lead %d", cfg.MinCandidateScore, cfg.MinCandidateLead), nil)
	if err := s.store.Save(s.snapshot); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.Refresh(ctx)
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

func (s *Service) RepairPerformer(ctx context.Context, performerID string) error {
	performerID = strings.TrimSpace(performerID)
	if performerID == "" {
		return fmt.Errorf("performer id is required")
	}
	if err := s.client.RepairPerformer(ctx, performerID); err != nil {
		return err
	}
	s.mu.Lock()
	s.appendAuditLocked("performer_repaired", fmt.Sprintf("attempted repair for performer %s", performerID), nil)
	if err := s.store.Save(s.snapshot); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.Refresh(ctx)
}

func (s *Service) SetReviewState(itemID string, state ReviewState) error {
	return s.SetReviewStateBulk([]string{itemID}, state)
}

func (s *Service) SetReviewStateBulk(itemIDs []string, state ReviewState) error {
	itemIDs = uniqueItemIDs(itemIDs)
	if len(itemIDs) == 0 {
		return fmt.Errorf("at least one item id is required")
	}
	if state != ReviewPending && state != ReviewSkipped {
		return fmt.Errorf("unsupported review state %q", state)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := 0
	now := s.now()
	for i := range s.snapshot.Items {
		if !containsItemID(itemIDs, s.snapshot.Items[i].ID) {
			continue
		}
		s.snapshot.Items[i].ReviewState = state
		s.snapshot.Items[i].ReviewedAt = now
		s.snapshot.Items[i].ResolvedAt = time.Time{}
		if state == ReviewPending {
			s.snapshot.Items[i].AssignedPerformerIDs = nil
		}
		found++
	}
	if found == 0 {
		return fmt.Errorf("no matching items found")
	}
	recountSnapshot(&s.snapshot)
	s.appendAuditLocked("review_state_updated", fmt.Sprintf("set review state to %s", state), itemIDs)
	if err := s.store.Save(s.snapshot); err != nil {
		return err
	}
	return nil
}

func (s *Service) AssignCandidate(ctx context.Context, itemID, performerID string) error {
	return s.AssignCandidateBulk(ctx, []string{itemID}, performerID)
}

func (s *Service) AssignCandidateBulk(ctx context.Context, itemIDs []string, performerID string) error {
	itemIDs = uniqueItemIDs(itemIDs)
	performerID = strings.TrimSpace(performerID)
	if len(itemIDs) == 0 || performerID == "" {
		return fmt.Errorf("item ids and performer id are required")
	}

	s.mu.RLock()
	items := make([]QueueItem, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		for _, item := range s.snapshot.Items {
			if item.ID == itemID {
				items = append(items, item)
				break
			}
		}
	}
	s.mu.RUnlock()

	if len(items) == 0 {
		return fmt.Errorf("no matching items found")
	}

	for _, item := range items {
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
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	updated := 0
	for i := range s.snapshot.Items {
		if !containsItemID(itemIDs, s.snapshot.Items[i].ID) {
			continue
		}
		s.snapshot.Items[i].ReviewState = ReviewResolved
		s.snapshot.Items[i].ReviewedAt = now
		s.snapshot.Items[i].ResolvedAt = now
		s.snapshot.Items[i].AssignedPerformerIDs = []string{performerID}
		updated++
	}
	if updated == 0 {
		return fmt.Errorf("no matching items found")
	}
	recountSnapshot(&s.snapshot)
	s.appendAuditLocked("assigned", fmt.Sprintf("assigned performer %s", performerID), itemIDs)
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
	snapshot.RepairCount = 0
	snapshot.SuppressedCount = 0
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
		} else if item.Status == "repair_needed" {
			snapshot.RepairCount++
		} else if item.Status == "suppressed" {
			snapshot.SuppressedCount++
		} else {
			snapshot.EmptyCount++
		}
	}
}

func (s *Service) appendAuditLocked(action, detail string, itemIDs []string) {
	entry := AuditEntry{
		At:            s.now(),
		Action:        action,
		Detail:        detail,
		ItemIDs:       append([]string{}, itemIDs...),
		MatchMinScore: s.match.MinCandidateScore,
		MatchMinLead:  s.match.MinCandidateLead,
	}
	appendAuditEntry(&s.snapshot, entry)
	s.snapshot.Settings = s.match.settings()
}

func appendAuditEntry(snapshot *Snapshot, entry AuditEntry) {
	snapshot.AuditTrail = append([]AuditEntry{entry}, snapshot.AuditTrail...)
	if len(snapshot.AuditTrail) > maxAuditEntries {
		snapshot.AuditTrail = snapshot.AuditTrail[:maxAuditEntries]
	}
}

func buildQueueItem(item stash.MediaItem, itemType ItemType, performerByID map[string]stash.Performer, completePerformers []stash.Performer, cfg matchConfig) (QueueItem, bool) {
	incompleteLinked := linkedIncompletePerformers(item.PerformerIDs, performerByID)
	if len(incompleteLinked) > 0 {
		return QueueItem{
			ID:               item.ID,
			Type:             itemType,
			Title:            item.Title,
			Details:          item.Details,
			Path:             item.Path,
			Tags:             append([]string{}, item.Tags...),
			Studio:           item.Studio,
			Status:           "repair_needed",
			ReviewState:      ReviewPending,
			LinkedPerformers: incompleteLinked,
		}, true
	}
	if len(item.PerformerIDs) > 0 {
		return QueueItem{}, false
	}
	return scoreItem(item, itemType, completePerformers, cfg), true
}

func linkedIncompletePerformers(performerIDs []string, performerByID map[string]stash.Performer) []LinkedPerformer {
	out := make([]LinkedPerformer, 0, len(performerIDs))
	for _, performerID := range performerIDs {
		performer, ok := performerByID[performerID]
		if !ok || !isIncompletePerformer(performer) {
			continue
		}
		stashIDs := make([]string, 0, len(performer.StashIDs))
		for _, stashID := range performer.StashIDs {
			stashIDs = append(stashIDs, stashID.Endpoint+":"+stashID.StashID)
		}
		out = append(out, LinkedPerformer{
			PerformerID: performer.ID,
			Name:        performer.Name,
			ImageURL:    performer.ImageURL,
			Gender:      performer.Gender,
			Incomplete:  true,
			CanRepair:   len(performer.StashIDs) > 0,
			StashIDs:    stashIDs,
		})
	}
	return out
}

func isIncompletePerformer(performer stash.Performer) bool {
	return strings.TrimSpace(performer.Name) == "" &&
		strings.TrimSpace(performer.Gender) == "" &&
		strings.TrimSpace(performer.ImageURL) == ""
}

func uniqueItemIDs(itemIDs []string) []string {
	out := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		trimmed := strings.TrimSpace(itemID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsItemID(itemIDs []string, target string) bool {
	for _, itemID := range itemIDs {
		if itemID == target {
			return true
		}
	}
	return false
}
