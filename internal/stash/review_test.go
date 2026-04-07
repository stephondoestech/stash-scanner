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

func TestMissingPerformerGalleriesUsesDirectPathWhenAvailable(t *testing.T) {
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
		if !strings.Contains(req.Query, " path ") && !strings.Contains(req.Query, "{ id title details path files") {
			t.Fatalf("expected direct gallery path query, got %q", req.Query)
		}
		return jsonResponse(`{"data":{"findGalleries":{"count":1,"galleries":[{"id":"g1","title":"Gallery","details":"","path":"/media/gallery-set","files":[],"performers":[],"tags":[],"studio":null}]}}}`), nil
	})}

	items, err := client.MissingPerformerGalleries(context.Background())
	if err != nil {
		t.Fatalf("MissingPerformerGalleries: %v", err)
	}
	if got, want := items[0].Path, "/media/gallery-set"; got != want {
		t.Fatalf("gallery path mismatch: got %q want %q", got, want)
	}
}

func TestMissingPerformerGalleriesFallsBackToFolderPathWhenDirectPathIsUnsupported(t *testing.T) {
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
			if !strings.Contains(req.Query, "path folder { path }") {
				t.Fatalf("expected direct path and folder query, got %q", req.Query)
			}
			return jsonResponse(`{"errors":[{"message":"Cannot query field \"path\" on type \"Gallery\"."}],"data":null}`), nil
		}

		if !strings.Contains(req.Query, "folder { path }") || strings.Contains(req.Query, "details path files") {
			t.Fatalf("expected folder fallback query without direct path, got %q", req.Query)
		}
		return jsonResponse(`{"data":{"findGalleries":{"count":1,"galleries":[{"id":"g1","title":"Gallery","details":"","folder":{"path":"/media/gallery-folder"},"files":[],"performers":[],"tags":[],"studio":null}]}}}`), nil
	})}

	items, err := client.MissingPerformerGalleries(context.Background())
	if err != nil {
		t.Fatalf("MissingPerformerGalleries: %v", err)
	}
	if got, want := items[0].Path, "/media/gallery-folder"; got != want {
		t.Fatalf("gallery path mismatch: got %q want %q", got, want)
	}
}

func TestAutoAssignGalleryPerformersFromScenePathsUsesFolderPathFallback(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	var mutations []string
	galleryRequests := 0
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch {
		case strings.Contains(req.Query, "findScenes"):
			return jsonResponse(`{"data":{"findScenes":{"count":1,"scenes":[{"id":"s1","title":"Scene","details":"","files":[{"path":"/media/shared-folder"}],"performers":[{"id":"p1"}],"tags":[],"studio":null}]}}}`), nil
		case strings.Contains(req.Query, "findGalleries"):
			galleryRequests++
			if galleryRequests == 1 {
				return jsonResponse(`{"errors":[{"message":"Cannot query field \"path\" on type \"Gallery\"."}],"data":null}`), nil
			}
			return jsonResponse(`{"data":{"findGalleries":{"count":1,"galleries":[{"id":"g1","title":"Gallery","details":"","folder":{"path":"/media/shared-folder"},"files":[],"performers":[],"tags":[],"studio":null}]}}}`), nil
		case strings.Contains(req.Query, "galleryUpdate"):
			mutations = append(mutations, req.Query)
			return jsonResponse(`{"data":{"galleryUpdate":{"id":"g1"}}}`), nil
		default:
			t.Fatalf("unexpected query %q", req.Query)
			return nil, nil
		}
	})}

	assigned, err := client.AutoAssignGalleryPerformersFromScenePaths(context.Background())
	if err != nil {
		t.Fatalf("AutoAssignGalleryPerformersFromScenePaths: %v", err)
	}
	if got, want := assigned, 1; got != want {
		t.Fatalf("assigned count mismatch: got %d want %d", got, want)
	}
	if got, want := len(mutations), 1; got != want {
		t.Fatalf("mutation count mismatch: got %d want %d", got, want)
	}
}

func TestAutoAssignGalleryPerformersFromScenePathsAssignsExactMatch(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	var mutations []string
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch {
		case strings.Contains(req.Query, "findScenes"):
			return jsonResponse(`{"data":{"findScenes":{"count":1,"scenes":[{"id":"s1","title":"Scene","details":"","files":[{"path":"/media/shared-path"}],"performers":[{"id":"p1"},{"id":"p2"}],"tags":[],"studio":null}]}}}`), nil
		case strings.Contains(req.Query, "findGalleries"):
			return jsonResponse(`{"data":{"findGalleries":{"count":1,"galleries":[{"id":"g1","title":"Gallery","details":"","path":"/media/shared-path","files":[],"performers":[],"tags":[],"studio":null}]}}}`), nil
		case strings.Contains(req.Query, "galleryUpdate"):
			mutations = append(mutations, req.Query)
			return jsonResponse(`{"data":{"galleryUpdate":{"id":"g1"}}}`), nil
		default:
			t.Fatalf("unexpected query %q", req.Query)
			return nil, nil
		}
	})}

	assigned, err := client.AutoAssignGalleryPerformersFromScenePaths(context.Background())
	if err != nil {
		t.Fatalf("AutoAssignGalleryPerformersFromScenePaths: %v", err)
	}
	if got, want := assigned, 1; got != want {
		t.Fatalf("assigned count mismatch: got %d want %d", got, want)
	}
	if got, want := len(mutations), 1; got != want {
		t.Fatalf("mutation count mismatch: got %d want %d", got, want)
	}
	if !strings.Contains(mutations[0], `performer_ids: ["p1", "p2"]`) {
		t.Fatalf("expected gallery update performer ids, got %q", mutations[0])
	}
}

func TestAssignScenePerformersSendsSceneUpdate(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	var query string
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		query = req.Query
		return jsonResponse(`{"data":{"sceneUpdate":{"id":"s1"}}}`), nil
	})}

	if err := client.AssignScenePerformers(context.Background(), "s1", []string{"p1", "p2"}); err != nil {
		t.Fatalf("AssignScenePerformers: %v", err)
	}
	if !strings.Contains(query, `sceneUpdate`) {
		t.Fatalf("expected scene update mutation, got %q", query)
	}
	if !strings.Contains(query, `performer_ids: ["p1", "p2"]`) {
		t.Fatalf("expected performer ids in scene update, got %q", query)
	}
}

func TestAssignGalleryPerformersSendsGalleryUpdate(t *testing.T) {
	client := NewClient("http://stash.local", "secret-key", false)
	var query string
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req gqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		query = req.Query
		return jsonResponse(`{"data":{"galleryUpdate":{"id":"g1"}}}`), nil
	})}

	if err := client.AssignGalleryPerformers(context.Background(), "g1", []string{"p1"}); err != nil {
		t.Fatalf("AssignGalleryPerformers: %v", err)
	}
	if !strings.Contains(query, `galleryUpdate`) {
		t.Fatalf("expected gallery update mutation, got %q", query)
	}
	if !strings.Contains(query, `performer_ids: ["p1"]`) {
		t.Fatalf("expected performer ids in gallery update, got %q", query)
	}
}
