package review

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"stash-scanner/internal/stash"
)

type fakeStashClient struct {
	scenes         []stash.MediaItem
	galleries      []stash.MediaItem
	performers     []stash.Performer
	image          stash.ImageResult
	autoAssigned   []stash.AutoAssignedGallery
	sceneAssigns   map[string][]string
	galleryAssigns map[string][]string
	repaired       []string
}

func (f *fakeStashClient) SceneItems(context.Context) ([]stash.MediaItem, error) {
	return f.scenes, nil
}

func (f *fakeStashClient) GalleryItems(context.Context) ([]stash.MediaItem, error) {
	return f.galleries, nil
}

func (f *fakeStashClient) Performers(context.Context) ([]stash.Performer, error) {
	return f.performers, nil
}

func (f *fakeStashClient) FetchImage(context.Context, string) (stash.ImageResult, error) {
	return f.image, nil
}

func (f *fakeStashClient) AutoAssignGalleryPerformersFromScenePaths(context.Context) ([]stash.AutoAssignedGallery, error) {
	return f.autoAssigned, nil
}

func (f *fakeStashClient) AssignScenePerformers(_ context.Context, sceneID string, performerIDs []string) error {
	if f.sceneAssigns == nil {
		f.sceneAssigns = map[string][]string{}
	}
	f.sceneAssigns[sceneID] = append([]string{}, performerIDs...)
	return nil
}

func (f *fakeStashClient) AssignGalleryPerformers(_ context.Context, galleryID string, performerIDs []string) error {
	if f.galleryAssigns == nil {
		f.galleryAssigns = map[string][]string{}
	}
	f.galleryAssigns[galleryID] = append([]string{}, performerIDs...)
	return nil
}

func (f *fakeStashClient) RepairPerformer(_ context.Context, performerID string) error {
	f.repaired = append(f.repaired, performerID)
	return nil
}

func TestRefreshBuildsReviewQueue(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{{
				ID:      "scene-1",
				Title:   "Jane Doe backstage",
				Path:    "/media/Jane Doe/scene.mp4",
				Tags:    []string{"interview"},
				Details: "Shot after the showcase",
			}},
			galleries: []stash.MediaItem{{
				ID:    "gallery-1",
				Title: "Unknown gallery",
				Path:  "/media/misc/gallery.zip",
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"},
				{ID: "perf-2", Name: "Alex Roe", Aliases: []string{"A Roe"}},
			},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.ItemCount, 2; got != want {
		t.Fatalf("item count mismatch: got %d want %d", got, want)
	}
	if got, want := status.ReviewCount, 1; got != want {
		t.Fatalf("review count mismatch: got %d want %d", got, want)
	}
	if got, want := status.EmptyCount, 1; got != want {
		t.Fatalf("empty count mismatch: got %d want %d", got, want)
	}
	if got, want := status.SuppressedCount, 0; got != want {
		t.Fatalf("suppressed count mismatch: got %d want %d", got, want)
	}
	if got := status.Items[0].Candidates[0].Name; got != "Jane Doe" {
		t.Fatalf("unexpected top candidate: %q", got)
	}
}

func TestRefreshTracksSuppressedItemsSeparately(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{{
				ID:     "scene-1",
				Title:  "Behind the scenes",
				Studio: "Jane Doe Productions",
				Tags:   []string{"Jane Doe"},
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe"},
			},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.SuppressedCount, 1; got != want {
		t.Fatalf("suppressed count mismatch: got %d want %d", got, want)
	}
	if got, want := status.EmptyCount, 0; got != want {
		t.Fatalf("empty count mismatch: got %d want %d", got, want)
	}
	if got, want := status.Items[0].Status, "suppressed"; got != want {
		t.Fatalf("item status mismatch: got %q want %q", got, want)
	}
}

func TestRefreshIncludesItemsWithIncompleteLinkedPerformers(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{{
				ID:           "scene-1",
				Title:        "Existing linked performer",
				Path:         "/media/scene.mp4",
				PerformerIDs: []string{"perf-1"},
			}},
			performers: []stash.Performer{{
				ID:       "perf-1",
				StashIDs: []stash.StashID{{Endpoint: "https://stashdb.org/graphql", StashID: "abc"}},
			}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.RepairCount, 1; got != want {
		t.Fatalf("repair count mismatch: got %d want %d", got, want)
	}
	if got, want := status.Items[0].Status, "repair_needed"; got != want {
		t.Fatalf("item status mismatch: got %q want %q", got, want)
	}
	if len(status.Items[0].LinkedPerformers) != 1 {
		t.Fatalf("expected linked incomplete performer, got %#v", status.Items[0].LinkedPerformers)
	}
}

func TestRefreshIncludesAutoAssignedGalleries(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			galleries: []stash.MediaItem{{
				ID:           "gallery-1",
				Title:        "Shared gallery",
				Path:         "/media/shared",
				PerformerIDs: []string{"perf-1", "perf-2"},
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe"},
				{ID: "perf-2", Name: "Alex Roe"},
			},
			autoAssigned: []stash.AutoAssignedGallery{{
				ID:           "gallery-1",
				Title:        "Shared gallery",
				Path:         "/media/shared",
				PerformerIDs: []string{"perf-1", "perf-2"},
				Reason:       "exact scene path match for /media/shared",
			}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.AutoAssignedCount, 1; got != want {
		t.Fatalf("auto-assigned count mismatch: got %d want %d", got, want)
	}
	if got, want := status.Items[0].Status, "auto_assigned"; got != want {
		t.Fatalf("item status mismatch: got %q want %q", got, want)
	}
	if !status.Items[0].AutoAssigned {
		t.Fatal("expected auto-assigned gallery flag")
	}
	if got, want := strings.Join(status.Items[0].AssignedPerformerIDs, ","), "perf-1,perf-2"; got != want {
		t.Fatalf("assigned performer ids mismatch: got %q want %q", got, want)
	}
}

func TestRefreshCarriesForwardAutoAssignedGalleries(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "queue.json"))
	initial := Snapshot{
		Items: []QueueItem{{
			ID:                   "gallery-1",
			Type:                 GalleryItem,
			Title:                "Shared gallery",
			Path:                 "/media/shared",
			Status:               "auto_assigned",
			AutoAssigned:         true,
			AutoAssignReason:     "exact scene path match for /media/shared",
			ReviewState:          ReviewResolved,
			AssignedPerformerIDs: []string{"perf-1", "perf-2"},
		}},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("save initial snapshot: %v", err)
	}

	service, err := NewService(
		store,
		&fakeStashClient{
			galleries: []stash.MediaItem{{
				ID:           "gallery-1",
				Title:        "Shared gallery",
				Path:         "/media/shared",
				PerformerIDs: []string{"perf-1", "perf-2"},
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe"},
				{ID: "perf-2", Name: "Alex Roe"},
			},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.ItemCount, 1; got != want {
		t.Fatalf("item count mismatch: got %d want %d", got, want)
	}
	if got, want := status.Items[0].Status, "auto_assigned"; got != want {
		t.Fatalf("item status mismatch: got %q want %q", got, want)
	}
}

func TestRepairPerformerRefreshesQueue(t *testing.T) {
	client := &fakeStashClient{
		scenes: []stash.MediaItem{{
			ID:           "scene-1",
			Title:        "Existing linked performer",
			Path:         "/media/scene.mp4",
			PerformerIDs: []string{"perf-1"},
		}},
		performers: []stash.Performer{{ID: "perf-1", StashIDs: []stash.StashID{{Endpoint: "https://stashdb.org/graphql", StashID: "abc"}}}},
	}
	service, err := NewService(NewStore(filepath.Join(t.TempDir(), "queue.json")), client, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	client.performers = []stash.Performer{{ID: "perf-1", Name: "Jane Doe", Gender: "FEMALE", ImageURL: "https://img/jane.jpg", StashIDs: []stash.StashID{{Endpoint: "https://stashdb.org/graphql", StashID: "abc"}}}}

	if err := service.RepairPerformer(context.Background(), "perf-1"); err != nil {
		t.Fatalf("RepairPerformer: %v", err)
	}
	if got, want := strings.Join(client.repaired, ","), "perf-1"; got != want {
		t.Fatalf("repaired mismatch: got %q want %q", got, want)
	}
	if got, want := service.Status().ItemCount, 0; got != want {
		t.Fatalf("expected repaired item to drop from queue, got %d", got)
	}
}

func TestCandidateImageLoadsFromClientWithoutPersistence(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{{
				ID:    "scene-1",
				Title: "Jane Doe backstage",
				Path:  "/media/Jane Doe/scene.mp4",
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"},
			},
			image: stash.ImageResult{
				ContentType: "image/jpeg",
				Data:        []byte("jpeg-bytes"),
			},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	image, err := service.CandidateImage(context.Background(), "scene-1", "perf-1")
	if err != nil {
		t.Fatalf("CandidateImage: %v", err)
	}
	if got, want := string(image.Data), "jpeg-bytes"; got != want {
		t.Fatalf("image bytes mismatch: got %q want %q", got, want)
	}
}

func TestRefreshPreservesSkippedReviewState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "queue.json"))
	initial := Snapshot{
		Items: []QueueItem{{
			ID:          "scene-1",
			Type:        SceneItem,
			Title:       "Jane Doe backstage",
			Status:      "needs_review",
			ReviewState: ReviewSkipped,
		}},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("save initial snapshot: %v", err)
	}

	service, err := NewService(
		store,
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/Jane Doe/scene.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.Items[0].ReviewState, ReviewSkipped; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
	if got, want := status.SkippedCount, 1; got != want {
		t.Fatalf("skipped count mismatch: got %d want %d", got, want)
	}
}

func TestSetReviewStatePersistsChange(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/Jane Doe/scene.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := service.SetReviewState("scene-1", ReviewSkipped); err != nil {
		t.Fatalf("SetReviewState: %v", err)
	}

	status := service.Status()
	if got, want := status.Items[0].ReviewState, ReviewSkipped; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
	if len(status.AuditTrail) == 0 {
		t.Fatal("expected audit entry after review state change")
	}
}

func TestAssignCandidateMarksSceneResolved(t *testing.T) {
	client := &fakeStashClient{
		scenes: []stash.MediaItem{{
			ID:    "scene-1",
			Title: "Jane Doe backstage",
			Path:  "/media/Jane Doe/scene.mp4",
		}},
		performers: []stash.Performer{
			{ID: "perf-1", Name: "Jane Doe"},
		},
	}
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		client,
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := service.AssignCandidate(context.Background(), "scene-1", "perf-1"); err != nil {
		t.Fatalf("AssignCandidate: %v", err)
	}

	status := service.Status()
	if got, want := status.Items[0].ReviewState, ReviewResolved; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
	if got, want := status.ResolvedCount, 1; got != want {
		t.Fatalf("resolved count mismatch: got %d want %d", got, want)
	}
	if got, want := status.ActiveCount, 0; got != want {
		t.Fatalf("active count mismatch: got %d want %d", got, want)
	}
	if got, want := strings.Join(client.sceneAssigns["scene-1"], ","), "perf-1"; got != want {
		t.Fatalf("scene assignment mismatch: got %q want %q", got, want)
	}
	if len(status.Items[0].AssignedPerformerIDs) != 1 || status.Items[0].AssignedPerformerIDs[0] != "perf-1" {
		t.Fatalf("assigned performer ids not persisted: %#v", status.Items[0].AssignedPerformerIDs)
	}
	if status.Items[0].ResolvedAt.IsZero() {
		t.Fatal("expected resolved timestamp")
	}
}

func TestRefreshPreservesResolvedReviewState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "queue.json"))
	initial := Snapshot{
		Items: []QueueItem{{
			ID:                   "scene-1",
			Type:                 SceneItem,
			Title:                "Jane Doe backstage",
			Status:               "needs_review",
			ReviewState:          ReviewResolved,
			AssignedPerformerIDs: []string{"perf-1"},
		}},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("save initial snapshot: %v", err)
	}

	service, err := NewService(
		store,
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/Jane Doe/scene.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	status := service.Status()
	if got, want := status.Items[0].ReviewState, ReviewResolved; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
	if got, want := status.ResolvedCount, 1; got != want {
		t.Fatalf("resolved count mismatch: got %d want %d", got, want)
	}
	if got, want := status.ActiveCount, 0; got != want {
		t.Fatalf("active count mismatch: got %d want %d", got, want)
	}
}

func TestSearchPerformersReturnsManualMatches(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{{
				ID:    "scene-1",
				Title: "Unknown scene",
				Path:  "/media/unknown.mp4",
			}},
			performers: []stash.Performer{
				{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"},
				{ID: "perf-2", Name: "Alex Roe", Aliases: []string{"A Roe"}},
			},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	results, err := service.SearchPerformers("jane")
	if err != nil {
		t.Fatalf("SearchPerformers: %v", err)
	}
	if got, want := len(results), 1; got != want {
		t.Fatalf("result count mismatch: got %d want %d", got, want)
	}
	if got, want := results[0].PerformerID, "perf-1"; got != want {
		t.Fatalf("performer id mismatch: got %q want %q", got, want)
	}
}

func TestUpdateMatchConfigPersistsAcrossRestart(t *testing.T) {
	queuePath := filepath.Join(t.TempDir(), "queue.json")
	service, err := NewService(
		NewStore(queuePath),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/jane.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.UpdateMatchConfig(context.Background(), matchConfig{MinCandidateScore: 12, MinCandidateLead: 5}); err != nil {
		t.Fatalf("UpdateMatchConfig: %v", err)
	}

	reloaded, err := NewService(
		NewStore(queuePath),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/jane.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService reload: %v", err)
	}

	status := reloaded.Status()
	if got, want := status.MatchMinScore, 12; got != want {
		t.Fatalf("min score mismatch: got %d want %d", got, want)
	}
	if got, want := status.MatchMinLead, 5; got != want {
		t.Fatalf("min lead mismatch: got %d want %d", got, want)
	}
	if len(status.AuditTrail) == 0 {
		t.Fatal("expected audit trail to persist")
	}
}

func TestSetReviewStateBulkUpdatesMultipleItems(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{
				{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/jane-1.mp4"},
				{ID: "scene-2", Title: "Jane Doe stage", Path: "/media/jane-2.mp4"},
			},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := service.SetReviewStateBulk([]string{"scene-1", "scene-2"}, ReviewSkipped); err != nil {
		t.Fatalf("SetReviewStateBulk: %v", err)
	}

	status := service.Status()
	if got, want := status.SkippedCount, 2; got != want {
		t.Fatalf("skipped count mismatch: got %d want %d", got, want)
	}
	if got, want := status.ActiveCount, 0; got != want {
		t.Fatalf("active count mismatch: got %d want %d", got, want)
	}
}

func TestAssignCandidateBulkMarksMixedItemsResolved(t *testing.T) {
	client := &fakeStashClient{
		scenes: []stash.MediaItem{
			{ID: "scene-1", Title: "Jane Doe backstage", Path: "/media/jane-1.mp4"},
		},
		galleries: []stash.MediaItem{
			{ID: "gallery-1", Title: "Unknown gallery", Path: "/media/gallery-1"},
		},
		performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
	}
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		client,
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := service.AssignCandidateBulk(context.Background(), []string{"scene-1", "gallery-1"}, "perf-1"); err != nil {
		t.Fatalf("AssignCandidateBulk: %v", err)
	}

	status := service.Status()
	if got, want := status.ResolvedCount, 2; got != want {
		t.Fatalf("resolved count mismatch: got %d want %d", got, want)
	}
	if got, want := strings.Join(client.sceneAssigns["scene-1"], ","), "perf-1"; got != want {
		t.Fatalf("scene assignment mismatch: got %q want %q", got, want)
	}
	if got, want := strings.Join(client.galleryAssigns["gallery-1"], ","), "perf-1"; got != want {
		t.Fatalf("gallery assignment mismatch: got %q want %q", got, want)
	}
}

func TestAssignPerformersSupportsMultiplePerformerIDs(t *testing.T) {
	client := &fakeStashClient{
		galleries: []stash.MediaItem{
			{ID: "gallery-1", Title: "Shared gallery", Path: "/media/gallery-1"},
		},
		performers: []stash.Performer{
			{ID: "perf-1", Name: "Jane Doe"},
			{ID: "perf-2", Name: "Alex Roe"},
		},
	}
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		client,
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := service.AssignPerformers(context.Background(), []string{"gallery-1"}, []string{"perf-1", "perf-2"}); err != nil {
		t.Fatalf("AssignPerformers: %v", err)
	}

	status := service.Status()
	if got, want := strings.Join(client.galleryAssigns["gallery-1"], ","), "perf-1,perf-2"; got != want {
		t.Fatalf("gallery assignment mismatch: got %q want %q", got, want)
	}
	if got, want := strings.Join(status.Items[0].AssignedPerformerIDs, ","), "perf-1,perf-2"; got != want {
		t.Fatalf("assigned performer ids mismatch: got %q want %q", got, want)
	}
}
