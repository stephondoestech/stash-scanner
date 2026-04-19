package review

import (
	"time"

	"stash-scanner/internal/stash"
)

func buildAutoAssignedItems(items []stash.AutoAssignedGallery) []QueueItem {
	queue := make([]QueueItem, 0, len(items))
	for _, item := range items {
		queue = append(queue, QueueItem{
			ID:                   item.ID,
			Type:                 GalleryItem,
			Title:                item.Title,
			Details:              item.Details,
			Path:                 item.Path,
			Tags:                 append([]string{}, item.Tags...),
			Studio:               item.Studio,
			Status:               "auto_assigned",
			AutoAssigned:         true,
			AutoAssignReason:     item.Reason,
			ReviewState:          ReviewResolved,
			AssignedPerformerIDs: append([]string{}, item.PerformerIDs...),
		})
	}
	return queue
}

func carryForwardAutoAssigned(previous []QueueItem, galleries []stash.MediaItem) []QueueItem {
	current := make(map[string]stash.MediaItem, len(galleries))
	for _, gallery := range galleries {
		current[gallery.ID] = gallery
	}

	carried := make([]QueueItem, 0)
	for _, item := range previous {
		if item.Type != GalleryItem || !item.AutoAssigned {
			continue
		}
		gallery, ok := current[item.ID]
		if !ok || len(gallery.PerformerIDs) == 0 {
			continue
		}
		next := item
		next.Title = gallery.Title
		next.Details = gallery.Details
		next.Path = gallery.Path
		next.Tags = append([]string{}, gallery.Tags...)
		next.Studio = gallery.Studio
		next.Status = "auto_assigned"
		next.AssignedPerformerIDs = append([]string{}, gallery.PerformerIDs...)
		carried = append(carried, next)
	}
	return carried
}

func uniqueQueueItems(items []QueueItem) []QueueItem {
	unique := make([]QueueItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := reviewKey(item.Type, item.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, item)
	}
	return unique
}

func applyDefaultResolvedAt(items []QueueItem, now time.Time) {
	for i := range items {
		if items[i].ReviewState != ReviewResolved {
			continue
		}
		if items[i].ReviewedAt.IsZero() {
			items[i].ReviewedAt = now
		}
		if items[i].ResolvedAt.IsZero() {
			items[i].ResolvedAt = now
		}
	}
}
