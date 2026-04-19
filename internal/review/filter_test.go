package review

import "testing"

func TestFilterQueueItemsMatchesTitlePathStudioAndTags(t *testing.T) {
	items := []QueueItem{
		{
			ID:     "scene-1",
			Type:   SceneItem,
			Title:  "Jane Doe scene",
			Path:   "/media/jane.mp4",
			Studio: "North Studio",
			Tags:   []string{"featured"},
		},
		{
			ID:     "scene-2",
			Type:   SceneItem,
			Title:  "Unknown scene",
			Path:   "/archive/other.mp4",
			Studio: "South Studio",
			Tags:   []string{"archive"},
		},
	}

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "title", query: "jane", want: "scene-1"},
		{name: "path", query: "/archive", want: "scene-2"},
		{name: "studio", query: "north", want: "scene-1"},
		{name: "tag", query: "archive", want: "scene-2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filtered := filterQueueItems(items, test.query)
			if got, want := len(filtered), 1; got != want {
				t.Fatalf("filtered length mismatch: got %d want %d", got, want)
			}
			if got, want := filtered[0].ID, test.want; got != want {
				t.Fatalf("filtered id mismatch: got %q want %q", got, want)
			}
		})
	}
}
