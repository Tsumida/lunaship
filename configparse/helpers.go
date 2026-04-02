package configparse

import (
	"strconv"
	"strings"
)

type FieldError struct {
	Path    string
	Message string
}

type Problems struct {
	details []FieldError
}

func (p *Problems) Add(path string, message string) {
	if p == nil {
		return
	}
	p.details = append(p.details, FieldError{
		Path:    path,
		Message: message,
	})
}

func (p *Problems) HasErrors() bool {
	return p != nil && len(p.details) > 0
}

func (p *Problems) Errors() []FieldError {
	if p == nil {
		return nil
	}
	out := make([]FieldError, len(p.details))
	copy(out, p.details)
	return out
}

func RequireTable(raw map[string]any, key string, path string, problems *Problems) (map[string]any, bool) {
	value, exists := raw[key]
	if !exists {
		problems.Add(path, "is required")
		return nil, false
	}

	table, ok := AsTable(value)
	if !ok {
		problems.Add(path, "must be a table")
		return nil, false
	}
	return table, true
}

func OptionalTable(raw map[string]any, key string, path string, problems *Problems) (map[string]any, bool) {
	value, ok := raw[key]
	if !ok {
		return nil, false
	}

	table, ok := AsTable(value)
	if !ok {
		problems.Add(path, "must be a table")
		return nil, false
	}
	return table, true
}

func AsTable(value any) (map[string]any, bool) {
	table, ok := value.(map[string]any)
	return table, ok
}

func OptionalString(raw map[string]any, key string, path string, problems *Problems) (string, bool) {
	value, ok := raw[key]
	if !ok {
		return "", false
	}

	str, ok := value.(string)
	if !ok {
		problems.Add(path, "must be a string")
		return "", false
	}
	return str, true
}

func OptionalBool(raw map[string]any, key string, path string, problems *Problems) (bool, bool) {
	value, ok := raw[key]
	if !ok {
		return false, false
	}

	parsed, ok := value.(bool)
	if !ok {
		problems.Add(path, "must be a boolean")
		return false, false
	}
	return parsed, true
}

func OptionalInt(raw map[string]any, key string, path string, problems *Problems) (int, bool) {
	value, ok := raw[key]
	if !ok {
		return 0, false
	}

	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		if typed > uint64(^uint(0)>>1) {
			problems.Add(path, "must fit in int")
			return 0, false
		}
		return int(typed), true
	default:
		problems.Add(path, "must be an integer")
		return 0, false
	}
}

func OptionalStringSlice(raw map[string]any, key string, path string, problems *Problems) ([]string, bool) {
	value, ok := raw[key]
	if !ok {
		return nil, false
	}

	switch values := value.(type) {
	case []string:
		return values, true
	case []any:
		out := make([]string, 0, len(values))
		for index, item := range values {
			str, ok := item.(string)
			if !ok {
				problems.Add(path+"["+strconv.Itoa(index)+"]", "must be a string")
				return nil, false
			}
			out = append(out, str)
		}
		return out, true
	default:
		problems.Add(path, "must be an array of strings")
		return nil, false
	}
}

func ValidatePort(port int, path string, problems *Problems) {
	if port < 1 || port > 65535 {
		problems.Add(path, "must be between 1 and 65535")
	}
}

func TrimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func OrderedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
