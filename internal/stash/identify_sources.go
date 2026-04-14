package stash

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"stash-scanner/internal/config"
)

func (c *Client) resolveIdentifyConfig(ctx context.Context, cfg config.PostScan) (config.PostScan, error) {
	if cfg.HasIdentifySources() {
		return cfg, nil
	}

	sources, err := c.discoverIdentifySources(ctx)
	if err != nil {
		return cfg, err
	}

	cfg.IdentifyStashBoxEndpoints = sources.stashBoxEndpoints
	cfg.IdentifyScraperIDs = sources.scraperIDs
	return cfg, nil
}

type identifySources struct {
	stashBoxEndpoints []string
	scraperIDs        []string
}

func (c *Client) discoverIdentifySources(ctx context.Context) (identifySources, error) {
	if c.url == "" || c.apiKey == "" {
		return identifySources{}, fmt.Errorf("stash_url and api_key are required to discover identify sources")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return identifySources{}, err
	}

	response, err := c.executeQuery(ctx, endpoint, identifySourceDiscoveryQuery())
	if err != nil {
		return identifySources{}, err
	}

	sources := identifySourcesFromDefaults(response)
	if len(sources.stashBoxEndpoints)+len(sources.scraperIDs) == 0 {
		return identifySources{}, fmt.Errorf("stash returned no default identify sources; configure STASH_SCANNER_IDENTIFY_* explicitly")
	}
	return sources, nil
}

func identifySourceDiscoveryQuery() string {
	return "query { configuration { general { stashBoxes { endpoint } } defaults { identify { sources { source { stash_box_endpoint scraper_id } } } } } listScrapers(types: [SCENE]) { id name scene { supported_scrapes } } }"
}

func identifySourcesFromDefaults(response gqlResponse) identifySources {
	sources := identifySources{}
	for _, item := range response.Data.Configuration.Defaults.Identify.Sources {
		if endpoint := strings.TrimSpace(item.Source.StashBoxEndpoint); endpoint != "" {
			sources.stashBoxEndpoints = append(sources.stashBoxEndpoints, endpoint)
		}
		if scraperID := strings.TrimSpace(item.Source.ScraperID); scraperID != "" {
			sources.scraperIDs = append(sources.scraperIDs, scraperID)
		}
	}
	sources.stashBoxEndpoints = uniqueNonEmpty(sources.stashBoxEndpoints)
	sources.scraperIDs = uniqueNonEmpty(sources.scraperIDs)
	return sources
}

func mapStashBoxEndpoints(stashBoxes []struct {
	Endpoint string `json:"endpoint"`
}) []string {
	endpoints := make([]string, 0, len(stashBoxes))
	for _, stashBox := range stashBoxes {
		endpoints = append(endpoints, stashBox.Endpoint)
	}
	return endpoints
}

func discoverFragmentSceneScrapers(scrapers []struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Scene *struct {
		SupportedScrapes []string `json:"supported_scrapes"`
	} `json:"scene"`
}) []string {
	type sceneScraper struct {
		id   string
		name string
	}

	found := make([]sceneScraper, 0, len(scrapers))
	for _, scraper := range scrapers {
		if scraper.Scene == nil || strings.TrimSpace(scraper.ID) == "" {
			continue
		}
		if !slices.Contains(scraper.Scene.SupportedScrapes, "FRAGMENT") {
			continue
		}
		found = append(found, sceneScraper{
			id:   strings.TrimSpace(scraper.ID),
			name: strings.TrimSpace(scraper.Name),
		})
	}

	slices.SortFunc(found, func(a, b sceneScraper) int {
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	ids := make([]string, 0, len(found))
	for _, scraper := range found {
		ids = append(ids, scraper.id)
	}
	return uniqueNonEmpty(ids)
}

func uniqueNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
