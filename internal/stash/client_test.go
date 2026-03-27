package stash

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTriggerScanSendsGraphQLRequest(t *testing.T) {
	var (
		gotContentType string
		gotAPIKey      string
		gotQuery       string
	)

	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotContentType = r.Header.Get("Content-Type")
		gotAPIKey = r.Header.Get("ApiKey")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotQuery = req.Query

		return jsonResponse(`{"data":{"metadataScan":"job-123"}}`), nil
	})}
	jobID, err := client.TriggerScan(context.Background(), []string{"/media/library/scene"})
	if err != nil {
		t.Fatalf("TriggerScan: %v", err)
	}
	if got, want := jobID, "job-123"; got != want {
		t.Fatalf("job id mismatch: got %q want %q", got, want)
	}

	if got, want := gotContentType, "application/json"; got != want {
		t.Fatalf("Content-Type mismatch: got %q want %q", got, want)
	}

	if got, want := gotAPIKey, "secret-key"; got != want {
		t.Fatalf("ApiKey mismatch: got %q want %q", got, want)
	}

	if !strings.Contains(gotQuery, "metadataScan") {
		t.Fatalf("expected metadataScan mutation, got %q", gotQuery)
	}

	if !strings.Contains(gotQuery, `"/media/library/scene"`) {
		t.Fatalf("expected path in query, got %q", gotQuery)
	}
}

func TestTriggerScanReturnsGraphQLError(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"errors":[{"message":"scan failed"}]}`), nil
	})}
	_, err := client.TriggerScan(context.Background(), []string{"/media/library/scene"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("expected GraphQL error message, got %v", err)
	}
}

func TestFindJobReturnsStatus(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"data":{"findJob":{"id":"job-123","status":"RUNNING","description":"Scanning","progress":0.5,"addTime":"2026-03-27T00:00:00Z","startTime":"2026-03-27T00:00:01Z","endTime":null,"error":""}}}`), nil
	})}

	job, err := client.FindJob(context.Background(), "job-123")
	if err != nil {
		t.Fatalf("FindJob: %v", err)
	}

	if got, want := job.Status, "RUNNING"; got != want {
		t.Fatalf("job status mismatch: got %q want %q", got, want)
	}
}

func TestStopJobSendsMutation(t *testing.T) {
	var gotQuery string

	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotQuery = req.Query
		return jsonResponse(`{"data":{"stopJob":true}}`), nil
	})}

	if err := client.StopJob(context.Background(), "job-123"); err != nil {
		t.Fatalf("StopJob: %v", err)
	}

	if !strings.Contains(gotQuery, `stopJob(job_id: "job-123")`) {
		t.Fatalf("expected stopJob mutation, got %q", gotQuery)
	}
}

func TestLibraryRootsReturnsPaths(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"data":{"configuration":{"general":{"stashes":[{"path":"/media/a"},{"path":"/media/b"}]}}}}`), nil
	})}

	roots, err := client.LibraryRoots(context.Background())
	if err != nil {
		t.Fatalf("LibraryRoots: %v", err)
	}

	if got, want := len(roots), 2; got != want {
		t.Fatalf("root count mismatch: got %d want %d", got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "base url",
			raw:  "http://localhost:9999",
			want: "http://localhost:9999/graphql",
		},
		{
			name: "existing graphql path",
			raw:  "http://localhost:9999/graphql",
			want: "http://localhost:9999/graphql",
		},
		{
			name: "playground path",
			raw:  "https://stash.local/playground",
			want: "https://stash.local/graphql",
		},
		{
			name: "nested path",
			raw:  "http://localhost:9999/stash",
			want: "http://localhost:9999/stash/graphql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEndpoint(tt.raw)
			if err != nil {
				t.Fatalf("normalizeEndpoint: %v", err)
			}
			if got != tt.want {
				t.Fatalf("endpoint mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}
