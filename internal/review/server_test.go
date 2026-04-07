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
