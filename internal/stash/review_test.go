package stash

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMissingPerformerScenesFiltersAssignedPerformers(t *testing.T) {
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
		if !strings.Contains(req.Query, "findScenes") {
			t.Fatalf("unexpected query %q", req.Query)
		}
		return jsonResponse(`{"data":{"findScenes":{"count":2,"scenes":[{"id":"1","title":"Jane Doe","details":"set","files":[{"path":"/media/jane.mp4"}],"performers":[],"tags":[{"name":"tag-a"}],"studio":{"name":"Studio A"}},{"id":"2","title":"Tagged","details":"","files":[],"performers":[{"id":"perf-1"}],"tags":[],"studio":null}]}}}`), nil
	})}

	items, err := client.MissingPerformerScenes(context.Background())
	if err != nil {
		t.Fatalf("MissingPerformerScenes: %v", err)
	}

	if got, want := len(items), 1; got != want {
		t.Fatalf("item count mismatch: got %d want %d", got, want)
	}
	if got := items[0].Studio; got != "Studio A" {
		t.Fatalf("unexpected studio %q", got)
	}
}

func TestPerformersReturnsAliasesAndImages(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"data":{"findPerformers":{"count":1,"performers":[{"id":"perf-1","name":"Jane Doe","aliases":["JD"],"image_path":"https://img/jane.jpg"}]}}}`), nil
	})}

	items, err := client.Performers(context.Background())
	if err != nil {
		t.Fatalf("Performers: %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("performer count mismatch: got %d want %d", got, want)
	}
	if got := items[0].ImageURL; got != "https://img/jane.jpg" {
		t.Fatalf("unexpected image URL %q", got)
	}
}

func TestPerformersFallsBackWhenAliasesFieldIsUnsupported(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	requests := 0
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if requests == 1 {
			if !strings.Contains(req.Query, "aliases") {
				t.Fatalf("expected aliases in first query, got %q", req.Query)
			}
			return jsonResponse(`{"errors":[{"message":"Cannot query field \"aliases\" on type \"Performer\"."}],"data":null}`), nil
		}

		if strings.Contains(req.Query, "aliases") {
			t.Fatalf("expected fallback query without aliases, got %q", req.Query)
		}
		return jsonResponse(`{"data":{"findPerformers":{"count":1,"performers":[{"id":"perf-1","name":"Jane Doe","image_path":"https://img/jane.jpg"}]}}}`), nil
	})}

	items, err := client.Performers(context.Background())
	if err != nil {
		t.Fatalf("Performers: %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("performer count mismatch: got %d want %d", got, want)
	}
	if got := len(items[0].Aliases); got != 0 {
		t.Fatalf("expected no aliases after fallback, got %d", got)
	}
}

func TestPerformersFallsBackWhenAliasesFieldIsRejectedWithHTTP422(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	requests := 0
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if requests == 1 {
			if !strings.Contains(req.Query, "aliases") {
				t.Fatalf("expected aliases in first query, got %q", req.Query)
			}
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"errors":[{"message":"Cannot query field \"aliases\" on type \"Performer\".","locations":[{"line":1,"column":100}],"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`,
				)),
			}, nil
		}

		if strings.Contains(req.Query, "aliases") {
			t.Fatalf("expected fallback query without aliases, got %q", req.Query)
		}
		return jsonResponse(`{"data":{"findPerformers":{"count":1,"performers":[{"id":"perf-1","name":"Jane Doe","image_path":"https://img/jane.jpg"}]}}}`), nil
	})}

	items, err := client.Performers(context.Background())
	if err != nil {
		t.Fatalf("Performers: %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("performer count mismatch: got %d want %d", got, want)
	}
}
