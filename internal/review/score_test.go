package review

import (
	"testing"

	"stash-scanner/internal/stash"
)

func TestScoreItemPrefersExactTitleAndPathMatches(t *testing.T) {
	item := stash.MediaItem{
		ID:    "scene-1",
		Title: "Jane Doe backstage",
		Path:  "/media/Jane Doe/scene.mp4",
	}
	performers := []stash.Performer{
		{ID: "perf-1", Name: "Jane Doe"},
		{ID: "perf-2", Name: "Alex Roe"},
	}

	scored := scoreItem(item, SceneItem, performers, defaultMatchConfig())
	if got, want := scored.Status, "needs_review"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got, want := len(scored.Candidates), 1; got != want {
		t.Fatalf("candidate count mismatch: got %d want %d", got, want)
	}
	if got, want := scored.Candidates[0].PerformerID, "perf-1"; got != want {
		t.Fatalf("top candidate mismatch: got %q want %q", got, want)
	}
	if scored.BestScore < defaultMatchConfig().MinCandidateScore {
		t.Fatalf("expected top score to clear threshold, got %d", scored.BestScore)
	}
}

func TestScoreItemRejectsStudioAndTagOnlySignal(t *testing.T) {
	item := stash.MediaItem{
		ID:     "scene-1",
		Title:  "Behind the scenes",
		Studio: "Jane Doe Productions",
		Tags:   []string{"Jane Doe"},
	}
	performers := []stash.Performer{
		{ID: "perf-1", Name: "Jane Doe"},
	}

	scored := scoreItem(item, SceneItem, performers, defaultMatchConfig())
	if got, want := scored.Status, "suppressed"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got, want := scored.SuppressionReason, "weak_signal"; got != want {
		t.Fatalf("suppression reason mismatch: got %q want %q", got, want)
	}
	if got := len(scored.Candidates); got != 0 {
		t.Fatalf("expected no candidates, got %d", got)
	}
}

func TestScoreItemRejectsRawSubstringFragments(t *testing.T) {
	item := stash.MediaItem{
		ID:    "scene-1",
		Title: "Anniversary special",
		Path:  "/media/collections/anniversary.mp4",
	}
	performers := []stash.Performer{
		{ID: "perf-1", Name: "Ann"},
	}

	scored := scoreItem(item, SceneItem, performers, defaultMatchConfig())
	if got, want := scored.Status, "no_candidate"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got := scored.SuppressionReason; got != "" {
		t.Fatalf("suppression reason mismatch: got %q want empty", got)
	}
}

func TestScoreItemSuppressesAmbiguousTopMatches(t *testing.T) {
	item := stash.MediaItem{
		ID:    "scene-1",
		Title: "Jane backstage",
		Path:  "/media/Jane/scene.mp4",
	}
	performers := []stash.Performer{
		{ID: "perf-1", Name: "Jane"},
		{ID: "perf-2", Name: "Jane", Aliases: []string{"J"}},
	}

	scored := scoreItem(item, SceneItem, performers, defaultMatchConfig())
	if got, want := scored.Status, "suppressed"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got, want := scored.SuppressionReason, "ambiguous_match"; got != want {
		t.Fatalf("suppression reason mismatch: got %q want %q", got, want)
	}
	if got := len(scored.Candidates); got != 0 {
		t.Fatalf("expected no candidates, got %d", got)
	}
}

func TestScoreItemAllowsTighterConfiguredThresholds(t *testing.T) {
	item := stash.MediaItem{
		ID:    "scene-1",
		Title: "Jane Doe backstage",
		Path:  "/media/jane-doe/scene.mp4",
	}
	performers := []stash.Performer{{ID: "perf-1", Name: "Jane Doe"}}

	scored := scoreItem(item, SceneItem, performers, matchConfig{MinCandidateScore: 24, MinCandidateLead: 3})
	if got, want := scored.Status, "suppressed"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got, want := scored.SuppressionReason, "weak_signal"; got != want {
		t.Fatalf("suppression reason mismatch: got %q want %q", got, want)
	}
}

func TestScoreItemRejectsSingleTokenAliasFragmentsInTags(t *testing.T) {
	item := stash.MediaItem{
		ID:   "scene-1",
		Tags: []string{"JD exclusive"},
	}
	performers := []stash.Performer{{ID: "perf-1", Name: "Jane Doe", Aliases: []string{"JD"}}}

	scored := scoreItem(item, SceneItem, performers, defaultMatchConfig())
	if got, want := scored.Status, "suppressed"; got != want {
		t.Fatalf("status mismatch: got %q want %q", got, want)
	}
	if got, want := scored.SuppressionReason, "weak_signal"; got != want {
		t.Fatalf("suppression reason mismatch: got %q want %q", got, want)
	}
}
