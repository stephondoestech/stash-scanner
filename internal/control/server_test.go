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

type fakeRunner struct {
	status  app.Status
	runErr  error
	stopErr error
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
