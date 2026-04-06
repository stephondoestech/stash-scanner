package review

import (
	"sort"
	"strings"
	"unicode"

	"stash-scanner/internal/stash"
)

func scoreItem(item stash.MediaItem, itemType ItemType, performers []stash.Performer) QueueItem {
	text := buildSearchText(item)
	candidates := make([]Candidate, 0, 5)

	for _, performer := range performers {
		score, reasons := scorePerformer(text, performer)
		if score == 0 {
			continue
		}
		candidates = append(candidates, Candidate{
			PerformerID: performer.ID,
			Name:        performer.Name,
			ImageURL:    performer.ImageURL,
			Aliases:     append([]string{}, performer.Aliases...),
			Score:       score,
			Reasons:     reasons,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Name < candidates[j].Name
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	status := "no_candidate"
	bestScore := 0
	if len(candidates) > 0 {
		status = "needs_review"
		bestScore = candidates[0].Score
	}

	return QueueItem{
		ID:           item.ID,
		Type:         itemType,
		Title:        item.Title,
		Details:      item.Details,
		Path:         item.Path,
		Tags:         append([]string{}, item.Tags...),
		Studio:       item.Studio,
		Status:       status,
		BestScore:    bestScore,
		CandidateCnt: len(candidates),
		Candidates:   candidates,
	}
}

func buildSearchText(item stash.MediaItem) string {
	parts := []string{item.Title, item.Path, item.Details, item.Studio}
	parts = append(parts, item.Tags...)
	return strings.ToLower(strings.Join(parts, " "))
}

func scorePerformer(text string, performer stash.Performer) (int, []string) {
	score := 0
	reasons := []string{}

	if name := strings.ToLower(strings.TrimSpace(performer.Name)); name != "" && strings.Contains(text, name) {
		score += 10
		reasons = append(reasons, "name match")
	}

	for _, alias := range performer.Aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" || !strings.Contains(text, alias) {
			continue
		}
		score += 6
		reasons = append(reasons, "alias match")
	}

	tokens := tokenize(performer.Name)
	for _, alias := range performer.Aliases {
		tokens = append(tokens, tokenize(alias)...)
	}

	seen := map[string]struct{}{}
	for _, token := range tokens {
		if len(token) < 3 || strings.Contains(text, token) == false {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		score++
	}
	if len(seen) > 0 {
		reasons = append(reasons, "token overlap")
	}

	return score, uniqueStrings(reasons)
}

func tokenize(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return uniqueStrings(fields)
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
