package version

import (
	"os"
	"path/filepath"
	"strings"
)

const fallback = "dev"

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
