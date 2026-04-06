package review

import (
	"sort"
	"strings"

	"stash-scanner/internal/stash"
)

func searchPerformers(query string, performers []stash.Performer) []Candidate {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	lowerQuery := strings.ToLower(query)
	queryTokens := tokenize(query)
	results := make([]Candidate, 0, 10)

	for _, performer := range performers {
		score, reasons := searchPerformer(lowerQuery, queryTokens, performer)
		if score == 0 {
			continue
		}
		results = append(results, Candidate{
			PerformerID: performer.ID,
			Name:        performer.Name,
			ImageURL:    performer.ImageURL,
			Aliases:     append([]string{}, performer.Aliases...),
			Score:       score,
			Reasons:     reasons,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Name < results[j].Name
	})
	if len(results) > 10 {
		results = results[:10]
	}
	return results
}

func searchPerformer(query string, queryTokens []string, performer stash.Performer) (int, []string) {
	score := 0
	reasons := []string{}

	name := strings.ToLower(strings.TrimSpace(performer.Name))
	if name != "" {
		switch {
		case name == query:
			score += 20
			reasons = append(reasons, "exact name")
		case strings.HasPrefix(name, query):
			score += 14
			reasons = append(reasons, "name prefix")
		case strings.Contains(name, query):
			score += 10
			reasons = append(reasons, "name contains")
		}
	}

	for _, alias := range performer.Aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		switch {
		case alias == query:
			score += 18
			reasons = append(reasons, "exact alias")
		case strings.HasPrefix(alias, query):
			score += 12
			reasons = append(reasons, "alias prefix")
		case strings.Contains(alias, query):
			score += 8
			reasons = append(reasons, "alias contains")
		}
	}

	if matches := overlapCount(queryTokens, append(tokenize(performer.Name), aliasTokens(performer.Aliases)...)); matches > 0 {
		score += matches * 3
		reasons = append(reasons, "token overlap")
	}

	return score, uniqueStrings(reasons)
}

func aliasTokens(aliases []string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		out = append(out, tokenize(alias)...)
	}
	return out
}

func overlapCount(left, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(right))
	for _, value := range right {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	matches := 0
	counted := map[string]struct{}{}
	for _, value := range left {
		if value == "" {
			continue
		}
		if _, ok := counted[value]; ok {
			continue
		}
		if _, ok := seen[value]; ok {
			counted[value] = struct{}{}
			matches++
		}
	}
	return matches
}
