package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type PostScan struct {
	Tasks                     []string `json:"tasks"`
	IdentifyStashBoxIndexes   []int    `json:"identify_stash_box_indexes"`
	IdentifyStashBoxEndpoints []string `json:"identify_stash_box_endpoints"`
	IdentifyScraperIDs        []string `json:"identify_scraper_ids"`
	CleanDryRun               bool     `json:"clean_dry_run"`
}

func applyPostScanEnv(cfg *Config) {
	overrideSlice(&cfg.PostScan.Tasks, "STASH_SCANNER_POST_SCAN_TASKS")
	overrideIntSlice(&cfg.PostScan.IdentifyStashBoxIndexes, "STASH_SCANNER_IDENTIFY_STASH_BOX_INDEXES")
	overrideSlice(&cfg.PostScan.IdentifyStashBoxEndpoints, "STASH_SCANNER_IDENTIFY_STASH_BOX_ENDPOINTS")
	overrideSlice(&cfg.PostScan.IdentifyScraperIDs, "STASH_SCANNER_IDENTIFY_SCRAPER_IDS")
	overrideBool(&cfg.PostScan.CleanDryRun, "STASH_SCANNER_POST_SCAN_CLEAN_DRY_RUN")
}

func (c *Config) validatePostScan() error {
	if len(c.PostScan.Tasks) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(c.PostScan.Tasks))
	seen := make(map[string]struct{}, len(c.PostScan.Tasks))
	for _, task := range c.PostScan.Tasks {
		name := normalizePostScanTask(task)
		switch name {
		case "auto_tag", "identify", "clean":
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			normalized = append(normalized, name)
		default:
			return fmt.Errorf("unsupported post_scan task %q", task)
		}
	}
	c.PostScan.Tasks = orderedPostScanTasks(normalized)

	for _, task := range c.PostScan.Tasks {
		if task == "identify" && !c.PostScan.HasIdentifySources() && (strings.TrimSpace(c.StashURL) == "" || strings.TrimSpace(c.APIKey) == "") {
			return fmt.Errorf("identify post_scan task requires configured identify sources or stash_url/api_key for source discovery")
		}
	}

	return nil
}

func (p PostScan) HasIdentifySources() bool {
	return len(p.IdentifyStashBoxIndexes) > 0 || len(p.IdentifyStashBoxEndpoints) > 0 || len(p.IdentifyScraperIDs) > 0
}

func orderedPostScanTasks(tasks []string) []string {
	ordered := make([]string, 0, len(tasks))
	for _, name := range []string{"identify", "auto_tag", "clean"} {
		for _, task := range tasks {
			if task == name {
				ordered = append(ordered, task)
			}
		}
	}
	return ordered
}

func normalizePostScanTask(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func overrideIntSlice(target *[]int, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}

	parts := strings.Split(value, ",")
	items := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		items = append(items, parsed)
	}
	*target = items
}
