package review

import "strings"

func filterQueueItems(items []QueueItem, query string) []QueueItem {
	query = normalizeFilterQuery(query)
	if query == "" {
		return append([]QueueItem{}, items...)
	}

	filtered := make([]QueueItem, 0, len(items))
	for _, item := range items {
		if queueItemMatches(item, query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func normalizeFilterQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func queueItemMatches(item QueueItem, query string) bool {
	fields := []string{
		item.ID,
		string(item.Type),
		item.Title,
		item.Details,
		item.Path,
		item.Studio,
		item.Status,
		string(item.ReviewState),
		item.AutoAssignReason,
		item.SuppressionReason,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	for _, candidate := range item.Candidates {
		if strings.Contains(strings.ToLower(candidate.Name), query) {
			return true
		}
	}
	for _, performer := range item.LinkedPerformers {
		if strings.Contains(strings.ToLower(performer.Name), query) {
			return true
		}
	}
	return false
}
