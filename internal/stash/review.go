package stash

import (
	"context"
	"fmt"
	"strings"
)

type MediaItem struct {
	ID      string
	Title   string
	Details string
	Path    string
	Tags    []string
	Studio  string
}

type Performer struct {
	ID       string
	Name     string
	Aliases  []string
	ImageURL string
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
			ID:      strings.TrimSpace(scene.ID),
			Title:   strings.TrimSpace(scene.Title),
			Details: strings.TrimSpace(scene.Details),
			Path:    firstPath(scene.Files),
			Tags:    tagNames(scene.Tags),
			Studio:  studioName(scene.Studio),
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
			ID:      strings.TrimSpace(gallery.ID),
			Title:   strings.TrimSpace(gallery.Title),
			Details: strings.TrimSpace(gallery.Details),
			Path:    firstPath(gallery.Files),
			Tags:    tagNames(gallery.Tags),
			Studio:  studioName(gallery.Studio),
		})
	}
	return items, nil
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

func (c *Client) allSceneItems(ctx context.Context) ([]struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Details string `json:"details"`
	Files   []struct {
		Path string `json:"path"`
	} `json:"files"`
	Performers []struct {
		ID string `json:"id"`
	} `json:"performers"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Studio *struct {
		Name string `json:"name"`
	} `json:"studio"`
}, error) {
	return collectMediaPages(ctx, c, buildFindScenesQuery, func(response gqlResponse) ([]struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Details string `json:"details"`
		Files   []struct {
			Path string `json:"path"`
		} `json:"files"`
		Performers []struct {
			ID string `json:"id"`
		} `json:"performers"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Studio *struct {
			Name string `json:"name"`
		} `json:"studio"`
	}, int) {
		return response.Data.FindScenes.Scenes, response.Data.FindScenes.Count
	})
}

func (c *Client) allGalleryItems(ctx context.Context) ([]struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Details string `json:"details"`
	Files   []struct {
		Path string `json:"path"`
	} `json:"files"`
	Performers []struct {
		ID string `json:"id"`
	} `json:"performers"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Studio *struct {
		Name string `json:"name"`
	} `json:"studio"`
}, error) {
	return collectMediaPages(ctx, c, buildFindGalleriesQuery, func(response gqlResponse) ([]struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Details string `json:"details"`
		Files   []struct {
			Path string `json:"path"`
		} `json:"files"`
		Performers []struct {
			ID string `json:"id"`
		} `json:"performers"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Studio *struct {
			Name string `json:"name"`
		} `json:"studio"`
	}, int) {
		return response.Data.FindGalleries.Galleries, response.Data.FindGalleries.Count
	})
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

func buildFindGalleriesQuery(page, perPage int) string {
	return fmt.Sprintf("query { findGalleries(filter: { page: %d, per_page: %d }) { count galleries { id title details files { path } performers { id } tags { name } studio { name } } } }", page, perPage)
}

func buildFindPerformersQuery(page, perPage int, includeAliases bool) string {
	fields := "id name image_path"
	if includeAliases {
		fields += " aliases"
	}
	return fmt.Sprintf("query { findPerformers(filter: { page: %d, per_page: %d }) { count performers { %s } } }", page, perPage, fields)
}

func firstPath(files []struct {
	Path string `json:"path"`
}) string {
	for _, file := range files {
		if path := strings.TrimSpace(file.Path); path != "" {
			return path
		}
	}
	return ""
}

func tagNames(tags []struct {
	Name string `json:"name"`
}) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func studioName(studio *struct {
	Name string `json:"name"`
}) string {
	if studio == nil {
		return ""
	}
	return strings.TrimSpace(studio.Name)
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
	return strings.Contains(message, "Cannot query field") && strings.Contains(message, `"`+field+`"`)
}
