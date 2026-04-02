package config

import (
	"fmt"
	"strings"

	"github.com/tsumida/lunaship/configparse"
)

type ErrorKind string

const (
	ErrorKindRead       ErrorKind = "read"
	ErrorKindParse      ErrorKind = "parse"
	ErrorKindValidation ErrorKind = "validation"
)

type ErrorDetail = configparse.FieldError

type LoadError struct {
	Kind    ErrorKind
	Source  string
	Err     error
	Details []ErrorDetail
}

func (e *LoadError) Error() string {
	if e == nil {
		return "<nil>"
	}

	parts := make([]string, 0, len(e.Details))
	for _, detail := range e.Details {
		if detail.Path == "" {
			parts = append(parts, detail.Message)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", detail.Path, detail.Message))
	}

	switch e.Kind {
	case ErrorKindValidation:
		if len(parts) == 0 {
			return "config validation failed"
		}
		return fmt.Sprintf("config validation failed: %s", strings.Join(parts, "; "))
	case ErrorKindRead:
		if e.Source == "" {
			return fmt.Sprintf("read config failed: %v", e.Err)
		}
		return fmt.Sprintf("read config %q failed: %v", e.Source, e.Err)
	case ErrorKindParse:
		return fmt.Sprintf("parse config failed: %v", e.Err)
	default:
		if e.Err != nil {
			return e.Err.Error()
		}
		return "config load failed"
	}
}

func (e *LoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
