package codexbar

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

func lowerASCII(value string) string { return strings.ToLower(value) }

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func asMap(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return nil
}

func asList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func lookup(mapped map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func pick(mapped map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := mapped[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	}
	return ""
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(typed), "%"), 64)
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	}
	return 0, false
}

func intPointer(value any) *int64 {
	switch typed := value.(type) {
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			if parsed >= 0 {
				return &parsed
			}
			return nil
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			if parsed >= 0 {
				return &parsed
			}
			return nil
		}
	}
	parsed, ok := floatValue(value)
	const int64Limit = 9223372036854775808.0
	if !ok || parsed < 0 || parsed != math.Trunc(parsed) || parsed >= int64Limit {
		return nil
	}
	result := int64(parsed)
	return &result
}

func timeValue(value any) *time.Time {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return &parsed
			}
		}
		if seconds, err := strconv.ParseFloat(text, 64); err == nil {
			return unixTime(seconds)
		}
	case json.Number:
		if seconds, err := typed.Float64(); err == nil {
			return unixTime(seconds)
		}
	case float64:
		return unixTime(typed)
	}
	return nil
}

func unixTime(seconds float64) *time.Time {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return nil
	}
	millis := seconds > 1e12
	if millis {
		seconds /= 1000
	}
	parsed := time.Unix(int64(seconds), 0).UTC()
	return &parsed
}
