package stash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"stash-scanner/internal/logging"
)

type Client struct {
	url    string
	apiKey string
	dryRun bool
	logger *log.Logger
	http   *http.Client
}

func NewClient(url, apiKey string, dryRun bool) *Client {
	return &Client{
		url:    strings.TrimSpace(url),
		apiKey: strings.TrimSpace(apiKey),
		dryRun: dryRun,
		logger: log.Default(),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) LibraryRoots(ctx context.Context) ([]string, error) {
	if c.url == "" || c.apiKey == "" {
		return nil, fmt.Errorf("stash_url and api_key are required to discover library roots")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return nil, err
	}

	response, err := c.executeQuery(ctx, endpoint, "query { configuration { general { stashes { path } } } }")
	if err != nil {
		return nil, err
	}

	roots := make([]string, 0, len(response.Data.Configuration.General.Stashes))
	seen := make(map[string]struct{}, len(response.Data.Configuration.General.Stashes))
	for _, stash := range response.Data.Configuration.General.Stashes {
		root := strings.TrimSpace(stash.Path)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("stash returned no library roots")
	}

	return roots, nil
}

func (c *Client) TriggerScan(ctx context.Context, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	if c.dryRun {
		logging.Event(c.logger, "stash_scan_dry_run", "targets", len(paths))
		return "", nil
	}

	if c.url == "" || c.apiKey == "" {
		return "", fmt.Errorf("stash_url and api_key are required when dry_run is false")
	}

	endpoint, err := normalizeEndpoint(c.url)
	if err != nil {
		return "", err
	}

	query, err := buildMetadataScanMutation(paths)
	if err != nil {
		return "", err
	}

	response, err := c.executeQuery(ctx, endpoint, query)
	if err != nil {
		return "", err
	}

	jobID, err := decodeJobID(response.Data.MetadataScan)
	if err != nil {
		return "", err
	}
	if jobID != "" {
		logging.Event(c.logger, "stash_scan_started", "targets", len(paths), "job_id", jobID)
		return jobID, nil
	}

	logging.Event(c.logger, "stash_scan_started", "targets", len(paths))
	return "", nil
}

type gqlRequest struct {
	Query string `json:"query"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlResponse struct {
	Data struct {
		MetadataScan  json.RawMessage `json:"metadataScan"`
		FindJob       *Job            `json:"findJob"`
		StopJob       bool            `json:"stopJob"`
		Configuration struct {
			General struct {
				Stashes []struct {
					Path string `json:"path"`
				} `json:"stashes"`
			} `json:"general"`
		} `json:"configuration"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

func (c *Client) executeQuery(ctx context.Context, endpoint, query string) (gqlResponse, error) {
	logging.DebugEvent(c.logger, "graphql_request", "endpoint", endpoint, "query", query)
	body, err := json.Marshal(gqlRequest{Query: query})
	if err != nil {
		return gqlResponse{}, fmt.Errorf("encode GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return gqlResponse{}, fmt.Errorf("build GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ApiKey", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return gqlResponse{}, fmt.Errorf("execute GraphQL request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return gqlResponse{}, fmt.Errorf("read GraphQL response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return gqlResponse{}, fmt.Errorf("stash HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	logging.DebugEvent(c.logger, "graphql_response", "endpoint", endpoint, "status", resp.StatusCode)

	var payload gqlResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return gqlResponse{}, fmt.Errorf("decode GraphQL response: %w", err)
	}

	if len(payload.Errors) > 0 {
		return gqlResponse{}, fmt.Errorf("stash GraphQL error: %s", payload.Errors[0].Message)
	}

	return payload, nil
}

func buildMetadataScanMutation(paths []string) (string, error) {
	items := make([]string, 0, len(paths))
	for _, currentPath := range paths {
		trimmed := strings.TrimSpace(currentPath)
		if trimmed == "" {
			continue
		}

		quoted, err := json.Marshal(trimmed)
		if err != nil {
			return "", fmt.Errorf("encode scan path %q: %w", trimmed, err)
		}
		items = append(items, string(quoted))
	}

	if len(items) == 0 {
		return "", fmt.Errorf("at least one scan path is required")
	}

	return "mutation { metadataScan(input: { paths: [" + strings.Join(items, ", ") + "] }) }", nil
}

func decodeJobID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var jobID string
	if err := json.Unmarshal(raw, &jobID); err != nil {
		return "", fmt.Errorf("decode metadataScan job id: %w", err)
	}
	return strings.TrimSpace(jobID), nil
}

func normalizeEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse stash_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("stash_url must be an absolute URL")
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/graphql"
		return parsed.String(), nil
	}

	if strings.HasSuffix(parsed.Path, "/playground") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/playground") + "/graphql"
		return parsed.String(), nil
	}

	if !strings.HasSuffix(parsed.Path, "/graphql") {
		parsed.Path = path.Join(parsed.Path, "graphql")
	}

	return parsed.String(), nil
}
