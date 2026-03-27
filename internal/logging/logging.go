package logging

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func Event(logger *log.Logger, event string, kv ...any) {
	if logger == nil {
		return
	}

	parts := []string{"event=" + formatValue(event)}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		parts = append(parts, key+"="+formatValue(kv[i+1]))
	}

	logger.Print(strings.Join(parts, " "))
}

func formatValue(value any) string {
	text := fmt.Sprint(value)
	if text == "" {
		return `""`
	}
	if strings.ContainsAny(text, " \t\n\r\"=") {
		return strconv.Quote(text)
	}
	return text
}
