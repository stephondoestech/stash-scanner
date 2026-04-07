package stash

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type MediaItem struct {
	ID           string
	Title        string
	Details      string
	Path         string
	Tags         []string
	Studio       string
	PerformerIDs []string
}

type Performer struct {
	ID       string
	Name     string
	Aliases  []string
	ImageURL string
}

type performerRef struct {
	ID string `json:"id"`
}

type tagRef struct {
	Name string `json:"name"`
}

type pathRef struct {
	Path string `json:"path"`
}

type studioRef struct {
	Name string `json:"name"`
}

type sceneRecord struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Details    string         `json:"details"`
	Files      []pathRef      `json:"files"`
	Performers []performerRef `json:"performers"`
	Tags       []tagRef       `json:"tags"`
	Studio     *studioRef     `json:"studio"`
}

type galleryRecord struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Details    string         `json:"details"`
	Path       string         `json:"path"`
	Folder     *pathRef       `json:"folder"`
	Files      []pathRef      `json:"files"`
	Performers []performerRef `json:"performers"`
	Tags       []tagRef       `json:"tags"`
	Studio     *studioRef     `json:"studio"`
}

func (c *Client) MissingPerformerScenes(ctx context.Context) ([]MediaItem, error) {
	raw, err := c.allSceneItems(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MediaItem, 0, len(raw))
	for _, scene := range raw {
		if len(scene.Performers) > 0 {
			continue
		}
		items = append(items, MediaItem{
			ID:           strings.TrimSpace(scene.ID),
			Title:        strings.TrimSpace(scene.Title),
			Details:      strings.TrimSpace(scene.Details),
			Path:         firstPath(scene.Files),
			Tags:         tagNames(scene.Tags),
			Studio:       studioName(scene.Studio),
			PerformerIDs: performerIDs(scene.Performers),
		})
	}
	return items, nil
}

func (c *Client) MissingPerformerGalleries(ctx context.Context) ([]MediaItem, error) {
	raw, err := c.allGalleryItems(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MediaItem, 0, len(raw))
	for _, gallery := range raw {
		if len(gallery.Performers) > 0 {
			continue
		}
		items = append(items, MediaItem{
			ID:           strings.TrimSpace(gallery.ID),
			Title:        strings.TrimSpace(gallery.Title),
			Details:      strings.TrimSpace(gallery.Details),
			Path:         galleryPath(gallery.Path, gallery.Folder, gallery.Files),
			Tags:         tagNames(gallery.Tags),
			Studio:       studioName(gallery.Studio),
			PerformerIDs: performerIDs(gallery.Performers),
		})
	}
	return items, nil
}

func (c *Client) AutoAssignGalleryPerformersFromScenePaths(ctx context.Context) (int, error) {
	scenes, err := c.allSceneItems(ctx)
	if err != nil {
		return 0, err
	}
	galleries, err := c.allGalleryItems(ctx)
	if err != nil {
		return 0, err
	}

	byPath := make(map[string][]string)
	ambiguous := make(map[string]struct{})
	for _, scene := range scenes {
		ids := performerIDs(scene.Performers)
		if len(ids) == 0 {
			continue
		}
		path := normalizeMediaPath(firstPath(scene.Files))
		if path == "" {
			continue
		}
		if existing, ok := byPath[path]; ok {
			if !slices.Equal(existing, ids) {
				ambiguous[path] = struct{}{}
			}
			continue
		}
		byPath[path] = ids
	}

	assigned := 0
	for _, gallery := range galleries {
		if len(gallery.Performers) > 0 {
			continue
		}
		path := normalizeMediaPath(galleryPath(gallery.Path, gallery.Folder, gallery.Files))
		if path == "" {
			continue
		}
		if _, bad := ambiguous[path]; bad {
			continue
		}
		ids := byPath[path]
		if len(ids) == 0 {
			continue
		}
		if err := c.AssignGalleryPerformers(ctx, strings.TrimSpace(gallery.ID), ids); err != nil {
			return assigned, err
		}
		assigned++
	}
	return assigned, nil
}

func (c *Client) Performers(ctx context.Context) ([]Performer, error) {
	if c.url == "" || c.apiKey == "" {
		return nil, fmt.Errorf("stash_url and api_key are required")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return nil, err
	}

	page := 1
	performers := []Performer{}
	includeAliases := true
	for {
		response, err := c.executeQuery(ctx, endpoint, buildFindPerformersQuery(page, 100, includeAliases))
		if err != nil {
			if includeAliases && isUnsupportedFieldError(err, "aliases") {
				includeAliases = false
				if page == 1 {
					continue
				}
			}
			return nil, err
		}
		for _, performer := range response.Data.FindPerformers.Performers {
			performers = append(performers, Performer{
				ID:       strings.TrimSpace(performer.ID),
				Name:     strings.TrimSpace(performer.Name),
				Aliases:  trimmedStrings(performer.Aliases),
				ImageURL: strings.TrimSpace(performer.ImagePath),
			})
		}
		if page*100 >= response.Data.FindPerformers.Count || len(response.Data.FindPerformers.Performers) == 0 {
			return performers, nil
		}
		page++
	}
}

func (c *Client) allSceneItems(ctx context.Context) ([]sceneRecord, error) {
	return collectMediaPages(ctx, c, buildFindScenesQuery, func(response gqlResponse) ([]sceneRecord, int) {
		return response.Data.FindScenes.Scenes, response.Data.FindScenes.Count
	})
}

func (c *Client) allGalleryItems(ctx context.Context) ([]galleryRecord, error) {
	queries := []string{
		"id title details path folder { path } files { path } performers { id } tags { name } studio { name }",
		"id title details folder { path } files { path } performers { id } tags { name } studio { name }",
		"id title details path files { path } performers { id } tags { name } studio { name }",
		"id title details files { path } performers { id } tags { name } studio { name }",
	}
	var lastErr error
	for _, fields := range queries {
		items, err := collectMediaPages(ctx, c, func(page, perPage int) string {
			return buildFindGalleriesQuery(page, perPage, fields)
		}, func(response gqlResponse) ([]galleryRecord, int) {
			return response.Data.FindGalleries.Galleries, response.Data.FindGalleries.Count
		})
		if err == nil {
			return items, nil
		}
		if isUnsupportedFieldError(err, "path") || isUnsupportedFieldError(err, "folder") {
			lastErr = err
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

func (c *Client) AssignGalleryPerformers(ctx context.Context, galleryID string, performerIDs []string) error {
	if strings.TrimSpace(galleryID) == "" {
		return fmt.Errorf("gallery id is required")
	}
	if len(performerIDs) == 0 {
		return fmt.Errorf("at least one performer id is required")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return err
	}
	response, err := c.executeQuery(ctx, endpoint, buildGalleryUpdateMutation(galleryID, performerIDs))
	if err != nil {
		return err
	}
	if response.Data.GalleryUpdate == nil || strings.TrimSpace(response.Data.GalleryUpdate.ID) == "" {
		return fmt.Errorf("gallery update returned no id")
	}
	return nil
}

func (c *Client) AssignScenePerformers(ctx context.Context, sceneID string, performerIDs []string) error {
	if strings.TrimSpace(sceneID) == "" {
		return fmt.Errorf("scene id is required")
	}
	if len(performerIDs) == 0 {
		return fmt.Errorf("at least one performer id is required")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return err
	}
	response, err := c.executeQuery(ctx, endpoint, buildSceneUpdateMutation(sceneID, performerIDs))
	if err != nil {
		return err
	}
	if response.Data.SceneUpdate == nil || strings.TrimSpace(response.Data.SceneUpdate.ID) == "" {
		return fmt.Errorf("scene update returned no id")
	}
	return nil
}

func buildGalleryUpdateMutation(galleryID string, performerIDs []string) string {
	items := make([]string, 0, len(performerIDs))
	for _, performerID := range performerIDs {
		trimmed := strings.TrimSpace(performerID)
		if trimmed == "" {
			continue
		}
		items = append(items, quoteString(trimmed))
	}
	return "mutation { galleryUpdate(input: { id: " + quoteString(strings.TrimSpace(galleryID)) + ", performer_ids: [" + strings.Join(items, ", ") + "] }) { id } }"
}

func buildSceneUpdateMutation(sceneID string, performerIDs []string) string {
	items := make([]string, 0, len(performerIDs))
	for _, performerID := range performerIDs {
		trimmed := strings.TrimSpace(performerID)
		if trimmed == "" {
			continue
		}
		items = append(items, quoteString(trimmed))
	}
	return "mutation { sceneUpdate(input: { id: " + quoteString(strings.TrimSpace(sceneID)) + ", performer_ids: [" + strings.Join(items, ", ") + "] }) { id } }"
}

func buildFindGalleriesQuery(page, perPage int, fields string) string {
	return fmt.Sprintf("query { findGalleries(filter: { page: %d, per_page: %d }) { count galleries { %s } } }", page, perPage, fields)
}

func galleryPath(path string, folder *pathRef, files []pathRef) string {
	if trimmed := strings.TrimSpace(path); trimmed != "" {
		return trimmed
	}
	if folder != nil {
		if trimmed := strings.TrimSpace(folder.Path); trimmed != "" {
			return trimmed
		}
	}
	return firstPath(files)
}

func normalizeMediaPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func collectMediaPages[T any](ctx context.Context, c *Client, buildQuery func(page, perPage int) string, unpack func(gqlResponse) ([]T, int)) ([]T, error) {
	if c.url == "" || c.apiKey == "" {
		return nil, fmt.Errorf("stash_url and api_key are required")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return nil, err
	}

	page := 1
	items := []T{}
	for {
		response, err := c.executeQuery(ctx, endpoint, buildQuery(page, 100))
		if err != nil {
			return nil, err
		}
		current, count := unpack(response)
		items = append(items, current...)
		if page*100 >= count || len(current) == 0 {
			return items, nil
		}
		page++
	}
}

func buildFindScenesQuery(page, perPage int) string {
	return fmt.Sprintf("query { findScenes(filter: { page: %d, per_page: %d }) { count scenes { id title details files { path } performers { id } tags { name } studio { name } } } }", page, perPage)
}

func buildFindPerformersQuery(page, perPage int, includeAliases bool) string {
	fields := "id name image_path"
	if includeAliases {
		fields += " aliases"
	}
	return fmt.Sprintf("query { findPerformers(filter: { page: %d, per_page: %d }) { count performers { %s } } }", page, perPage, fields)
}

func firstPath(files []pathRef) string {
	for _, file := range files {
		if path := strings.TrimSpace(file.Path); path != "" {
			return path
		}
	}
	return ""
}

func tagNames(tags []tagRef) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func studioName(studio *studioRef) string {
	if studio == nil {
		return ""
	}
	return strings.TrimSpace(studio.Name)
}

func performerIDs(performers []performerRef) []string {
	ids := make([]string, 0, len(performers))
	for _, performer := range performers {
		if id := strings.TrimSpace(performer.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func trimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func isUnsupportedFieldError(err error, field string) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Cannot query field") && (strings.Contains(message, `"`+field+`"`) || strings.Contains(message, `\"`+field+`\"`))
}
