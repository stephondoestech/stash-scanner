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
	Gender   string
	Aliases  []string
	ImageURL string
	StashIDs []StashID
}

type StashID struct {
	Endpoint string
	StashID  string
}

type scrapedPerformerRecord struct {
	Name         string   `json:"name"`
	Gender       string   `json:"gender"`
	URLs         []string `json:"urls"`
	Aliases      string   `json:"aliases"`
	Images       []string `json:"images"`
	Details      string   `json:"details"`
	Birthdate    string   `json:"birthdate"`
	Ethnicity    string   `json:"ethnicity"`
	Country      string   `json:"country"`
	EyeColor     string   `json:"eye_color"`
	Measurements string   `json:"measurements"`
	Tattoos      string   `json:"tattoos"`
	Piercings    string   `json:"piercings"`
	CareerStart  string   `json:"career_start"`
	CareerEnd    string   `json:"career_end"`
	DeathDate    string   `json:"death_date"`
	HairColor    string   `json:"hair_color"`
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

func (c *Client) SceneItems(ctx context.Context) ([]MediaItem, error) {
	raw, err := c.allSceneItems(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MediaItem, 0, len(raw))
	for _, scene := range raw {
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

func (c *Client) MissingPerformerScenes(ctx context.Context) ([]MediaItem, error) {
	items, err := c.SceneItems(ctx)
	if err != nil {
		return nil, err
	}
	return filterMissingPerformerItems(items), nil
}

func (c *Client) GalleryItems(ctx context.Context) ([]MediaItem, error) {
	raw, err := c.allGalleryItems(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MediaItem, 0, len(raw))
	for _, gallery := range raw {
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

func (c *Client) MissingPerformerGalleries(ctx context.Context) ([]MediaItem, error) {
	items, err := c.GalleryItems(ctx)
	if err != nil {
		return nil, err
	}
	return filterMissingPerformerItems(items), nil
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
				Gender:   strings.TrimSpace(performer.Gender),
				Aliases:  trimmedStrings(performer.Aliases),
				ImageURL: strings.TrimSpace(performer.ImagePath),
				StashIDs: stashIDsFromRefs(performer.StashIDs),
			})
		}
		if page*100 >= response.Data.FindPerformers.Count || len(response.Data.FindPerformers.Performers) == 0 {
			return performers, nil
		}
		page++
	}
}

func (c *Client) RepairPerformer(ctx context.Context, performerID string) error {
	performerID = strings.TrimSpace(performerID)
	if performerID == "" {
		return fmt.Errorf("performer id is required")
	}
	performer, err := c.findPerformerByID(ctx, performerID)
	if err != nil {
		return err
	}
	source, err := preferredPerformerSource(performer)
	if err != nil {
		return err
	}
	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return err
	}
	response, err := c.executeQuery(ctx, endpoint, buildScrapeSinglePerformerQuery(performerID, source))
	if err != nil {
		return err
	}
	if len(response.Data.ScrapeSinglePerformer) == 0 {
		return fmt.Errorf("stash returned no scraped performer data")
	}
	updateMutation, err := buildPerformerRepairMutation(performerID, performer, response.Data.ScrapeSinglePerformer[0])
	if err != nil {
		return err
	}
	updateResponse, err := c.executeQuery(ctx, endpoint, updateMutation)
	if err != nil {
		return err
	}
	if updateResponse.Data.PerformerUpdate == nil || strings.TrimSpace(updateResponse.Data.PerformerUpdate.ID) == "" {
		return fmt.Errorf("performer update returned no id")
	}
	return nil
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
	fields := "id name gender image_path stash_ids { endpoint stash_id }"
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

func filterMissingPerformerItems(items []MediaItem) []MediaItem {
	out := make([]MediaItem, 0, len(items))
	for _, item := range items {
		if len(item.PerformerIDs) == 0 {
			out = append(out, item)
		}
	}
	return out
}

func stashIDsFromRefs(values []struct {
	Endpoint string `json:"endpoint"`
	StashID  string `json:"stash_id"`
}) []StashID {
	out := make([]StashID, 0, len(values))
	for _, value := range values {
		endpoint := strings.TrimSpace(value.Endpoint)
		stashID := strings.TrimSpace(value.StashID)
		if endpoint == "" || stashID == "" {
			continue
		}
		out = append(out, StashID{Endpoint: endpoint, StashID: stashID})
	}
	return out
}

func (c *Client) findPerformerByID(ctx context.Context, performerID string) (Performer, error) {
	if c.url == "" || c.apiKey == "" {
		return Performer{}, fmt.Errorf("stash_url and api_key are required")
	}
	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return Performer{}, err
	}
	response, err := c.executeQuery(ctx, endpoint, buildFindPerformerQuery(performerID))
	if err != nil {
		return Performer{}, err
	}
	if strings.TrimSpace(response.Data.FindPerformer.ID) == "" {
		return Performer{}, fmt.Errorf("performer %q not found", performerID)
	}
	return Performer{
		ID:       strings.TrimSpace(response.Data.FindPerformer.ID),
		Name:     strings.TrimSpace(response.Data.FindPerformer.Name),
		Gender:   strings.TrimSpace(response.Data.FindPerformer.Gender),
		Aliases:  trimmedStrings(response.Data.FindPerformer.Aliases),
		ImageURL: strings.TrimSpace(response.Data.FindPerformer.ImagePath),
		StashIDs: stashIDsFromRefs(response.Data.FindPerformer.StashIDs),
	}, nil
}

func preferredPerformerSource(performer Performer) (string, error) {
	if len(performer.StashIDs) == 0 {
		return "", fmt.Errorf("performer %q has no stash ids to repair from", performer.ID)
	}
	preferred := []string{"stashdb.org", "fansdb.cc", "theporndb.net"}
	for _, target := range preferred {
		for _, stashID := range performer.StashIDs {
			if strings.Contains(strings.ToLower(stashID.Endpoint), target) {
				return stashID.Endpoint, nil
			}
		}
	}
	return performer.StashIDs[0].Endpoint, nil
}

func buildFindPerformerQuery(performerID string) string {
	return "query { findPerformer(id: " + quoteString(strings.TrimSpace(performerID)) + ") { id name gender aliases image_path stash_ids { endpoint stash_id } } }"
}

func buildScrapeSinglePerformerQuery(performerID, sourceEndpoint string) string {
	return "query { scrapeSinglePerformer(source: { stash_box_endpoint: " + quoteString(strings.TrimSpace(sourceEndpoint)) + " }, input: { performer_id: " + quoteString(strings.TrimSpace(performerID)) + " }) { name gender urls aliases images details birthdate ethnicity country eye_color measurements tattoos piercings career_start career_end death_date hair_color } }"
}

func buildPerformerRepairMutation(performerID string, existing Performer, scraped scrapedPerformerRecord) (string, error) {
	parts := []string{"id: " + quoteString(strings.TrimSpace(performerID))}
	if value := strings.TrimSpace(scraped.Name); value != "" {
		parts = append(parts, "name: "+quoteString(value))
	}
	if value := normalizeGender(scraped.Gender); value != "" {
		parts = append(parts, "gender: "+value)
	}
	if urls := encodeQuotedStrings(scraped.URLs); urls != "" {
		parts = append(parts, "urls: ["+urls+"]")
	}
	if aliases := parseAliasList(scraped.Aliases); len(aliases) > 0 {
		parts = append(parts, "alias_list: ["+encodeQuotedStrings(aliases)+"]")
	}
	if image := firstNonEmpty(scraped.Images); image != "" {
		parts = append(parts, "image: "+quoteString(image))
	}
	if value := strings.TrimSpace(scraped.Details); value != "" {
		parts = append(parts, "details: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Birthdate); value != "" {
		parts = append(parts, "birthdate: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Ethnicity); value != "" {
		parts = append(parts, "ethnicity: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Country); value != "" {
		parts = append(parts, "country: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.EyeColor); value != "" {
		parts = append(parts, "eye_color: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Measurements); value != "" {
		parts = append(parts, "measurements: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Tattoos); value != "" {
		parts = append(parts, "tattoos: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.Piercings); value != "" {
		parts = append(parts, "piercings: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.CareerStart); value != "" {
		parts = append(parts, "career_start: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.CareerEnd); value != "" {
		parts = append(parts, "career_end: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.DeathDate); value != "" {
		parts = append(parts, "death_date: "+quoteString(value))
	}
	if value := strings.TrimSpace(scraped.HairColor); value != "" {
		parts = append(parts, "hair_color: "+quoteString(value))
	}
	if len(parts) == 1 {
		return "", fmt.Errorf("scraped performer returned no repairable fields for %q", existing.ID)
	}
	return "mutation { performerUpdate(input: { " + strings.Join(parts, ", ") + " }) { id } }", nil
}

func normalizeGender(value string) string {
	value = strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(value, "-", "_")))
	switch value {
	case "MALE", "FEMALE", "TRANSGENDER_MALE", "TRANSGENDER_FEMALE", "INTERSEX", "NON_BINARY":
		return value
	default:
		return ""
	}
}

func parseAliasList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	return trimmedStrings(parts)
}

func encodeQuotedStrings(values []string) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			items = append(items, quoteString(trimmed))
		}
	}
	return strings.Join(items, ", ")
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isUnsupportedFieldError(err error, field string) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Cannot query field") && (strings.Contains(message, `"`+field+`"`) || strings.Contains(message, `\"`+field+`\"`))
}
