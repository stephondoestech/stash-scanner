package stash

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ImageResult struct {
	ContentType string
	Data        []byte
}

func (c *Client) FetchImage(ctx context.Context, rawURL string) (ImageResult, error) {
	if strings.TrimSpace(rawURL) == "" {
		return ImageResult{}, fmt.Errorf("image URL is required")
	}

	target, err := c.resolveAssetURL(rawURL)
	if err != nil {
		return ImageResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ImageResult{}, fmt.Errorf("build image request: %w", err)
	}
	if shouldAttachAPIKey(c.url, target) && c.apiKey != "" {
		req.Header.Set("ApiKey", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ImageResult{}, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ImageResult{}, fmt.Errorf("image HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, fmt.Errorf("read image: %w", err)
	}

	return ImageResult{
		ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
		Data:        data,
	}, nil
}

func (c *Client) resolveAssetURL(raw string) (string, error) {
	asset, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse image URL: %w", err)
	}
	if asset.Scheme != "" && asset.Host != "" {
		return asset.String(), nil
	}

	base, err := url.Parse(strings.TrimSpace(c.url))
	if err != nil {
		return "", fmt.Errorf("parse stash_url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("stash_url must be an absolute URL")
	}

	return base.ResolveReference(asset).String(), nil
}

func shouldAttachAPIKey(stashURL, target string) bool {
	stashParsed, err := url.Parse(strings.TrimSpace(stashURL))
	if err != nil {
		return false
	}
	targetParsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return false
	}
	return strings.EqualFold(stashParsed.Host, targetParsed.Host)
}
