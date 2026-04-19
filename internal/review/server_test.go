package review

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"stash-scanner/internal/stash"
)

func TestStatusHandlerReturnsQueue(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{performers: []stash.Performer{}},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	var payload Status
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ItemCount != 0 {
		t.Fatalf("expected empty queue, got %d items", payload.ItemCount)
	}
	if payload.MatchMinScore == 0 || payload.MatchMinLead < 0 {
		t.Fatalf("expected active match config in status, got %+v", payload)
	}
	if got, want := payload.VisibleCount, 0; got != want {
		t.Fatalf("visible count mismatch: got %d want %d", got, want)
	}
}

func TestStatusHandlerFiltersQueueByQuery(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{
				{ID: "scene-1", Title: "Jane Doe scene", Path: "/media/jane.mp4", Studio: "North Studio"},
				{ID: "scene-2", Title: "Unknown scene", Path: "/vault/other.mp4", Studio: "South Studio"},
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

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodGet, "/api/status?q=north", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	var payload Status
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := payload.ItemCount, 2; got != want {
		t.Fatalf("item count mismatch: got %d want %d", got, want)
	}
	if got, want := payload.VisibleCount, 1; got != want {
		t.Fatalf("visible count mismatch: got %d want %d", got, want)
	}
	if got, want := payload.FilterQuery, "north"; got != want {
		t.Fatalf("filter query mismatch: got %q want %q", got, want)
	}
	if got, want := len(payload.Items), 1; got != want {
		t.Fatalf("filtered item count mismatch: got %d want %d", got, want)
	}
	if got, want := payload.Items[0].ID, "scene-1"; got != want {
		t.Fatalf("filtered item mismatch: got %q want %q", got, want)
	}
}

func TestRefreshHandlerRunsRefresh(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe scene"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := service.Status().ItemCount, 1; got != want {
		t.Fatalf("item count mismatch: got %d want %d", got, want)
	}
}

func TestCandidateImageHandlerReturnsNoStoreImage(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe scene"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"}},
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

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodGet, "/api/candidate-image?item_id=scene-1&performer_id=perf-1", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "no-store, max-age=0"; got != want {
		t.Fatalf("cache header mismatch: got %q want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "image/jpeg"; got != want {
		t.Fatalf("content-type mismatch: got %q want %q", got, want)
	}
}

func TestItemStateHandlerUpdatesReviewState(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe scene"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/items/state", strings.NewReader(`{"item_id":"scene-1","review_state":"skipped"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := service.Status().Items[0].ReviewState, ReviewSkipped; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
}

func TestAssignHandlerMarksItemResolved(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe scene", Path: "/media/jane.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/items/assign", strings.NewReader(`{"item_id":"scene-1","performer_id":"perf-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	status := service.Status()
	if got, want := status.Items[0].ReviewState, ReviewResolved; got != want {
		t.Fatalf("review state mismatch: got %q want %q", got, want)
	}
	if got, want := status.ResolvedCount, 1; got != want {
		t.Fatalf("resolved count mismatch: got %d want %d", got, want)
	}
}

func TestPerformerSearchHandlerReturnsMatches(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Unknown scene", Path: "/media/unknown.mp4"}},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodGet, "/api/performers/search?q=jane", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	var payload []Candidate
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := len(payload), 1; got != want {
		t.Fatalf("payload count mismatch: got %d want %d", got, want)
	}
	if got, want := payload[0].PerformerID, "perf-1"; got != want {
		t.Fatalf("performer id mismatch: got %q want %q", got, want)
	}
}

func TestBulkStateHandlerUpdatesMultipleItems(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{
				{ID: "scene-1", Title: "Jane Doe scene", Path: "/media/jane-1.mp4"},
				{ID: "scene-2", Title: "Jane Roe scene", Path: "/media/jane-2.mp4"},
			},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/items/state-bulk", strings.NewReader(`{"item_ids":["scene-1","scene-2"],"review_state":"skipped"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := service.Status().SkippedCount, 2; got != want {
		t.Fatalf("skipped count mismatch: got %d want %d", got, want)
	}
}

func TestBulkAssignHandlerMarksMultipleItemsResolved(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes: []stash.MediaItem{
				{ID: "scene-1", Title: "Jane Doe scene", Path: "/media/jane-1.mp4"},
			},
			galleries: []stash.MediaItem{
				{ID: "gallery-1", Title: "Jane Doe gallery", Path: "/media/gallery-1"},
			},
			performers: []stash.Performer{{ID: "perf-1", Name: "Jane Doe", ImageURL: "https://img/jane.jpg"}},
		},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/items/assign-bulk", strings.NewReader(`{"item_ids":["scene-1","gallery-1"],"performer_id":"perf-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := service.Status().ResolvedCount, 2; got != want {
		t.Fatalf("resolved count mismatch: got %d want %d", got, want)
	}
}

func TestMultiAssignHandlerAssignsMultiplePerformers(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			galleries: []stash.MediaItem{
				{ID: "gallery-1", Title: "Jane gallery", Path: "/media/gallery-1"},
			},
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

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/items/assign-multi", strings.NewReader(`{"item_ids":["gallery-1"],"performer_ids":["perf-1","perf-2"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := strings.Join(service.Status().Items[0].AssignedPerformerIDs, ","), "perf-1,perf-2"; got != want {
		t.Fatalf("assigned performer ids mismatch: got %q want %q", got, want)
	}
}

func TestSettingsHandlerUpdatesReviewerThresholds(t *testing.T) {
	service, err := NewService(
		NewStore(filepath.Join(t.TempDir(), "queue.json")),
		&fakeStashClient{
			scenes:     []stash.MediaItem{{ID: "scene-1", Title: "Jane Doe scene", Path: "/media/jane.mp4"}},
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

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{"min_score":24,"min_lead":4}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	status := service.Status()
	if got, want := status.MatchMinScore, 24; got != want {
		t.Fatalf("min score mismatch: got %d want %d", got, want)
	}
	if got, want := status.MatchMinLead, 4; got != want {
		t.Fatalf("min lead mismatch: got %d want %d", got, want)
	}
}

func TestRepairPerformerHandlerRunsRepair(t *testing.T) {
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

	server := NewServer("127.0.0.1:0", service, log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/api/performers/repair", strings.NewReader(`{"performer_id":"perf-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
	if got, want := strings.Join(client.repaired, ","), "perf-1"; got != want {
		t.Fatalf("repair call mismatch: got %q want %q", got, want)
	}
}
