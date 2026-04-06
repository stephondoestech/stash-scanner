package review

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"

	"stash-scanner/internal/stash"
)

type fakeStashClient struct {
	scenes     []stash.MediaItem
	galleries  []stash.MediaItem
	performers []stash.Performer
}

func (f *fakeStashClient) MissingPerformerScenes(context.Context) ([]stash.MediaItem, error) {
	return f.scenes, nil
}

func (f *fakeStashClient) MissingPerformerGalleries(context.Context) ([]stash.MediaItem, error) {
	return f.galleries, nil
}

func (f *fakeStashClient) Performers(context.Context) ([]stash.Performer, error) {
	return f.performers, nil
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
	if got := status.Items[0].Candidates[0].Name; got != "Jane Doe" {
		t.Fatalf("unexpected top candidate: %q", got)
	}
}
