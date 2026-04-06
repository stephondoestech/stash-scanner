package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"stash-scanner/internal/app"
	"stash-scanner/internal/review"
	"stash-scanner/internal/stash"
	"stash-scanner/internal/state"
)

func TestStatusHandlerReturnsJSON(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{
		status: app.Status{
			Now:     time.Date(2026, time.March, 26, 12, 0, 0, 0, time.UTC),
			Running: true,
			PendingScan: state.PendingScan{
				Paths: []string{"/media/a"},
			},
		},
	}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	var payload app.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !payload.Running {
		t.Fatal("expected running status")
	}
}

func TestRunNowHandlerReturnsConflictWhenBusy(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{runErr: app.ErrRunInProgress}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/run-now", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusConflict; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	if !strings.Contains(rec.Body.String(), "running") {
		t.Fatalf("expected running body, got %q", rec.Body.String())
	}
}

func TestStopHandlerReturnsAccepted(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/stop", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	if !strings.Contains(rec.Body.String(), "stopping") {
		t.Fatalf("expected stopping body, got %q", rec.Body.String())
	}
}

func TestStopHandlerReturnsConflictWhenIdle(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{stopErr: app.ErrNoRunInProgress}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/stop", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusConflict; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
}

func TestFlushDebounceHandlerReturnsAccepted(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/flush-debounce", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	if !strings.Contains(rec.Body.String(), "flushing") {
		t.Fatalf("expected flushing body, got %q", rec.Body.String())
	}
}

func TestFlushDebounceHandlerReturnsConflictWhenIdle(t *testing.T) {
	server := New("127.0.0.1:0", "", &fakeRunner{flushErr: app.ErrNoPendingDebounce}, log.New(io.Discard, "", 0))

	req := httptest.NewRequest(http.MethodPost, "/api/flush-debounce", nil)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusConflict; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}
}

func TestFormatBindErrorAddressInUse(t *testing.T) {
	err := formatBindError("127.0.0.1:8088", syscall.EADDRINUSE)
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatBindErrorPermissionDenied(t *testing.T) {
	err := formatBindError("127.0.0.1:8088", syscall.EPERM)
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatBindErrorGeneric(t *testing.T) {
	err := formatBindError("127.0.0.1:8088", errors.New("boom"))
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMountReviewerServesMountedRoutes(t *testing.T) {
	service, err := review.NewService(
		review.NewStore(t.TempDir()+"/queue.json"),
		&fakeReviewClient{performers: []stash.Performer{}},
		log.New(io.Discard, "", 0),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	server := New("127.0.0.1:0", "", &fakeRunner{}, log.New(io.Discard, "", 0))
	server.MountReviewer(service)

	indexReq := httptest.NewRequest(http.MethodGet, "/reviewer/", nil)
	indexRec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(indexRec, indexReq)
	if got, want := indexRec.Code, http.StatusOK; got != want {
		t.Fatalf("reviewer index status mismatch: got %d want %d", got, want)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/reviewer/api/status", nil)
	statusRec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(statusRec, statusReq)
	if got, want := statusRec.Code, http.StatusOK; got != want {
		t.Fatalf("reviewer status mismatch: got %d want %d", got, want)
	}
}

type fakeRunner struct {
	status   app.Status
	runErr   error
	stopErr  error
	flushErr error
}

type fakeReviewClient struct {
	scenes     []stash.MediaItem
	galleries  []stash.MediaItem
	performers []stash.Performer
}

func (f *fakeRunner) Status(context.Context) (app.Status, error) {
	return f.status, nil
}

func (f *fakeRunner) StartManualRun() error {
	return f.runErr
}

func (f *fakeRunner) StopActiveRun(context.Context) error {
	return f.stopErr
}

func (f *fakeRunner) FlushPendingDebounce() error {
	return f.flushErr
}

func (f *fakeReviewClient) MissingPerformerScenes(context.Context) ([]stash.MediaItem, error) {
	return f.scenes, nil
}

func (f *fakeReviewClient) MissingPerformerGalleries(context.Context) ([]stash.MediaItem, error) {
	return f.galleries, nil
}

func (f *fakeReviewClient) Performers(context.Context) ([]stash.Performer, error) {
	return f.performers, nil
}
