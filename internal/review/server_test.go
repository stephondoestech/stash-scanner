package review

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
