package review

import (
	"sort"
	"strings"
	"unicode"

	"stash-scanner/internal/stash"
)

type matchConfig struct {
	MinCandidateScore int
	MinCandidateLead  int
}

func (c matchConfig) settings() MatchSettings {
	return MatchSettings{
		MinScore: c.MinCandidateScore,
		MinLead:  c.MinCandidateLead,
	}
}

type scoreInput struct {
	title       string
	path        string
	details     string
	studio      string
	tags        []string
	titleTokens []string
	pathTokens  []string
}

func defaultMatchConfig() matchConfig {
	return matchConfig{
		MinCandidateScore: 8,
		MinCandidateLead:  3,
	}
}

func scoreItem(item stash.MediaItem, itemType ItemType, performers []stash.Performer, cfg matchConfig) QueueItem {
	input := buildScoreInput(item)
	candidates := make([]Candidate, 0, 5)
	lowSignal := false

	for _, performer := range performers {
		score, reasons := scorePerformer(input, performer)
		if score > 0 && score < cfg.MinCandidateScore {
			lowSignal = true
		}
		if score < cfg.MinCandidateScore {
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
	suppressionReason := ""
	if len(candidates) > 0 && !ambiguousTopCandidate(candidates, cfg) {
		status = "needs_review"
		bestScore = candidates[0].Score
	} else {
		if len(candidates) > 0 {
			status = "suppressed"
			suppressionReason = "ambiguous_match"
		} else if lowSignal {
			status = "suppressed"
			suppressionReason = "weak_signal"
		} else {
			suppressionReason = ""
		}
		candidates = nil
	}

	return QueueItem{
		ID:                item.ID,
		Type:              itemType,
		Title:             item.Title,
		Details:           item.Details,
		Path:              item.Path,
		Tags:              append([]string{}, item.Tags...),
		Studio:            item.Studio,
		Status:            status,
		SuppressionReason: suppressionReason,
		BestScore:         bestScore,
		CandidateCnt:      len(candidates),
		Candidates:        candidates,
	}
}

func buildScoreInput(item stash.MediaItem) scoreInput {
	title := normalizeMatchText(item.Title)
	path := normalizeMatchText(item.Path)
	return scoreInput{
		title:       title,
		path:        path,
		details:     normalizeMatchText(item.Details),
		studio:      normalizeMatchText(item.Studio),
		tags:        normalizeMatchTextSlice(item.Tags),
		titleTokens: tokenize(title),
		pathTokens:  tokenize(path),
	}
}

func scorePerformer(input scoreInput, performer stash.Performer) (int, []string) {
	score := 0
	reasons := []string{}

	if phrase := normalizeMatchText(performer.Name); phrase != "" {
		score, reasons = addPhraseScore(score, reasons, phrase, "name", input, 10, 9, 4, 2, 2)
	}

	for _, alias := range performer.Aliases {
		phrase := normalizeMatchText(alias)
		if phrase == "" {
			continue
		}
		score, reasons = addPhraseScore(score, reasons, phrase, "alias", input, 8, 7, 3, 2, 2)
	}

	tokens := tokenize(performer.Name)
	for _, alias := range performer.Aliases {
		tokens = append(tokens, tokenize(alias)...)
	}
	tokenMatches := overlapCount(tokens, append(input.titleTokens, input.pathTokens...))
	if tokenMatches >= 2 {
		score += tokenMatches * 2
		reasons = append(reasons, "title/path token overlap")
	}

	return score, uniqueStrings(reasons)
}

func addPhraseScore(score int, reasons []string, phrase, label string, input scoreInput, titleWeight, pathWeight, detailWeight, studioWeight, tagWeight int) (int, []string) {
	if containsPhrase(input.title, phrase) {
		score += weightedPhraseScore(phrase, titleWeight)
		reasons = append(reasons, "title "+label+" match")
	}
	if containsPhrase(input.path, phrase) {
		score += weightedPhraseScore(phrase, pathWeight)
		reasons = append(reasons, "path "+label+" match")
	}
	if containsPhrase(input.details, phrase) {
		score += weightedPhraseScore(phrase, detailWeight)
		reasons = append(reasons, "details "+label+" match")
	}
	if containsPhrase(input.studio, phrase) {
		score += weightedPhraseScore(phrase, studioWeight)
		reasons = append(reasons, "studio "+label+" match")
	}
	for _, tag := range input.tags {
		if containsPhrase(tag, phrase) {
			score += weightedPhraseScore(phrase, tagWeight)
			reasons = append(reasons, "tag "+label+" match")
			break
		}
	}
	return score, reasons
}

func weightedPhraseScore(phrase string, weight int) int {
	if len(tokenize(phrase)) <= 1 {
		return maxInt(1, weight/2)
	}
	return weight
}

func containsPhrase(field, phrase string) bool {
	if field == "" || phrase == "" {
		return false
	}
	return strings.Contains(" "+field+" ", " "+phrase+" ")
}

func normalizeMatchText(value string) string {
	return strings.Join(tokenize(value), " ")
}

func normalizeMatchTextSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeMatchText(value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func ambiguousTopCandidate(candidates []Candidate, cfg matchConfig) bool {
	if len(candidates) < 2 {
		return false
	}
	return candidates[0].Score-candidates[1].Score < cfg.MinCandidateLead
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

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
