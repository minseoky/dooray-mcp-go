package tools

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
)

// requireOperation rejects an input whose operation does not match the tool.
func requireOperation(input map[string]any, expected string) error {
	if operation, _ := input["operation"].(string); operation != expected {
		return fmt.Errorf("operation must be %s", expected)
	}
	return nil
}

// requireString reads a mandatory non-empty string field.
func requireString(input map[string]any, key string) (string, error) {
	value, ok := input[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return value, nil
}

// optionalString reads a string field, returning "" when it is absent or of
// another type.
func optionalString(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return value
}

// objectField reads a nested object field.
func objectField(input map[string]any, key string) (map[string]any, bool) {
	value, ok := input[key].(map[string]any)
	return value, ok
}

// pick copies the given keys into query parameters, skipping values that are
// absent, null, or empty strings.
func pick(input map[string]any, keys []string) url.Values {
	query := url.Values{}
	for _, key := range keys {
		if text, ok := queryValue(input[key]); ok {
			query.Set(key, text)
		}
	}
	return query
}

// set adds one query parameter unless the value is absent, null, or an empty
// string.
func set(query url.Values, key string, value any) {
	if text, ok := queryValue(value); ok {
		query.Set(key, text)
	}
}

// queryValue renders a JSON value the way String() would in JavaScript, and
// reports whether it should appear in the query string at all.
func queryValue(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		return formatNumber(typed), true
	case int:
		return strconv.Itoa(typed), true
	default:
		return fmt.Sprint(typed), true
	}
}

// formatNumber prints whole numbers without a trailing ".0", matching the
// JavaScript number-to-string conversion for the values Dooray expects.
func formatNumber(value float64) string {
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// coalesce mirrors the JavaScript `??` operator: the fallback is used only
// when the field is absent or null.
func coalesce(input map[string]any, key string, fallback any) any {
	value, exists := input[key]
	if !exists || value == nil {
		return fallback
	}
	return value
}

// confirmDescription is shared by every write tool so the confirmation
// parameter reads the same way wherever it appears.
const confirmDescription = "must be true to execute this write operation; set it only after the user has confirmed the change"

// requireConfirmation rejects a write call that does not carry an explicit
// confirm=true. Input validation alone is not enough for a tool that mutates
// Dooray state, so the caller has to opt in on every single call.
func requireConfirmation(input map[string]any) error {
	confirmed, ok := input["confirm"].(bool)
	if !ok || !confirmed {
		return fmt.Errorf("confirm must be true to execute this write operation")
	}
	return nil
}
