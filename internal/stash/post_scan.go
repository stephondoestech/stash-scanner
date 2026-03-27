package stash

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stash-scanner/internal/config"
	"stash-scanner/internal/logging"
)

type PostScanTask string

const (
	PostScanAutoTag  PostScanTask = "auto_tag"
	PostScanIdentify PostScanTask = "identify"
	PostScanClean    PostScanTask = "clean"
)

func (c *Client) TriggerPostScanTask(ctx context.Context, task PostScanTask, paths []string, cfg config.PostScan) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	if c.dryRun {
		logging.Event(c.logger, "stash_post_scan_dry_run", "task", task, "targets", len(paths))
		return "", nil
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return "", err
	}

	query, err := buildPostScanMutation(task, paths, cfg)
	if err != nil {
		return "", err
	}

	response, err := c.executeQuery(ctx, endpoint, query)
	if err != nil {
		return "", err
	}

	jobID, err := decodePostScanJobID(response, task)
	if err != nil {
		return "", err
	}
	logging.Event(c.logger, "stash_post_scan_started", "task", task, "targets", len(paths), "job_id", jobID)
	return jobID, nil
}

func buildPostScanMutation(task PostScanTask, paths []string, cfg config.PostScan) (string, error) {
	encodedPaths, err := encodeStringList(paths)
	if err != nil {
		return "", err
	}

	switch task {
	case PostScanAutoTag:
		return "mutation { metadataAutoTag(input: { paths: " + encodedPaths + " }) }", nil
	case PostScanClean:
		return fmt.Sprintf("mutation { metadataClean(input: { paths: %s, dryRun: %t }) }", encodedPaths, cfg.CleanDryRun), nil
	case PostScanIdentify:
		sources, err := encodeIdentifySources(cfg)
		if err != nil {
			return "", err
		}
		return "mutation { metadataIdentify(input: { paths: " + encodedPaths + ", sources: " + sources + " }) }", nil
	default:
		return "", fmt.Errorf("unsupported post-scan task %q", task)
	}
}

func decodePostScanJobID(response gqlResponse, task PostScanTask) (string, error) {
	switch task {
	case PostScanAutoTag:
		return decodeJobID(response.Data.MetadataAutoTag)
	case PostScanIdentify:
		return decodeJobID(response.Data.MetadataIdentify)
	case PostScanClean:
		return decodeJobID(response.Data.MetadataClean)
	default:
		return "", fmt.Errorf("unsupported post-scan task %q", task)
	}
}

func encodeIdentifySources(cfg config.PostScan) (string, error) {
	items := make([]string, 0, len(cfg.IdentifyStashBoxIndexes)+len(cfg.IdentifyStashBoxEndpoints)+len(cfg.IdentifyScraperIDs))
	for _, index := range cfg.IdentifyStashBoxIndexes {
		items = append(items, fmt.Sprintf("{ source: { stash_box_index: %d } }", index))
	}
	for _, endpoint := range cfg.IdentifyStashBoxEndpoints {
		items = append(items, "{ source: { stash_box_endpoint: "+quoteString(strings.TrimSpace(endpoint))+" } }")
	}
	for _, scraperID := range cfg.IdentifyScraperIDs {
		items = append(items, "{ source: { scraper_id: "+quoteString(strings.TrimSpace(scraperID))+" } }")
	}
	if len(items) == 0 {
		return "", fmt.Errorf("identify task requires at least one source")
	}
	return "[" + strings.Join(items, ", ") + "]", nil
}

func encodeStringList(values []string) (string, error) {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		quoted, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("encode string %q: %w", value, err)
		}
		items = append(items, string(quoted))
	}
	if len(items) == 0 {
		return "", fmt.Errorf("at least one path is required")
	}
	return "[" + strings.Join(items, ", ") + "]", nil
}
