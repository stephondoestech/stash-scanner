package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fallback = "dev"

var (
	commit = ""
)

func Current() string {
	candidates := []string{
		"VERSION",
		filepath.Join("/app", "VERSION"),
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			return value
		}
	}

	return fallback
}

func Commit() string {
	value := strings.TrimSpace(commit)
	if value == "" {
		return "unknown"
	}
	return value
}

func Describe() string {
	current := Current()
	currentCommit := Commit()
	if currentCommit == "unknown" {
		return current
	}
	return fmt.Sprintf("%s (%s)", current, currentCommit)
}
