package configparse

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
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

func Decode(input any, out any) (*mapstructure.Metadata, error) {
	metadata := &mapstructure.Metadata{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:   out,
		Metadata: metadata,
		TagName:  "toml",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(input); err != nil {
		return nil, err
	}
	return metadata, nil
}

func AddDecodeError(problems *Problems, path string, err error) {
	if err == nil {
		return
	}
	if path == "" {
		problems.Add("", err.Error())
		return
	}
	problems.Add(path, err.Error())
}

func AddUnused(problems *Problems, metadata *mapstructure.Metadata, ignorePaths map[string]struct{}) {
	if metadata == nil {
		return
	}
	for _, path := range metadata.Unused {
		if _, ok := ignorePaths[path]; ok {
			continue
		}
		problems.Add(path, "unknown field")
	}
}

func ValidatePort(port int, path string, problems *Problems) {
	if port < 1 || port > 65535 {
		problems.Add(path, "must be between 1 and 65535")
	}
}

func ValidateDuration(value time.Duration, path string, problems *Problems) {
	if value < 0 {
		problems.Add(path, "must be greater than or equal to 0")
	}
}

func TrimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func LookupPath(root map[string]any, path string) (any, error) {
	normalized := strings.TrimSpace(path)
	normalized = strings.TrimPrefix(normalized, ".")
	if normalized == "" {
		return nil, fmt.Errorf("config section must not be empty")
	}

	parts := strings.Split(normalized, ".")
	current := any(root)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid config section %q", path)
		}

		table, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config section %q is not a table path", path)
		}

		next, ok := table[part]
		if !ok {
			return nil, fmt.Errorf("config section %q not found", path)
		}
		current = next
	}

	return current, nil
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
