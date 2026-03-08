package log

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequiredFieldsCore(t *testing.T) {
	t.Run("flow: inject required defaults when caller does not provide fields", func(t *testing.T) {
		// Description: app/env and trace defaults come from env and should be injected into every record.
		// Expectation: required common fields always exist even for plain logger.Info calls.
		t.Setenv("APP_NAME", "unit-app")
		t.Setenv("APP_ENV", "test")
		t.Setenv("APP_IP", "10.1.2.3")
		t.Setenv("APP_PORT", "8088")

		logger, observed := newObservedLogger(defaultInjectedFieldsFromEnv())
		logger.Info("hello")

		entries := observed.All()
		assert.Len(t, entries, 1)
		ctx := entries[0].ContextMap()
		assert.Equal(t, "test", ctx["_env"])
		assert.Equal(t, "unit-app", ctx["_app"])
		assert.Equal(t, "10.1.2.3", ctx["_app_ip"])
		assert.EqualValues(t, 8088, ctx["_app_port"])
		assert.Equal(t, "", ctx["_trace_id"])
		assert.Equal(t, "", ctx["_span_id"])
	})

	t.Run("flow: truncate long message and sql fields", func(t *testing.T) {
		// Description: log body and sql text should be bounded before writing.
		// Expectation: both _msg and _sql are truncated to 1024 UTF-8 characters.
		logger, observed := newObservedLogger([]zap.Field{
			zap.String("_env", "test"),
			zap.String("_app", "unit-app"),
			zap.String("_app_ip", ""),
			zap.Int("_app_port", 0),
			zap.String("_trace_id", ""),
			zap.String("_span_id", ""),
		})
		longText := strings.Repeat("世", maxLogTextLen+32)
		logger.Info(longText, zap.String("_sql", longText))

		entries := observed.All()
		assert.Len(t, entries, 1)
		assert.Equal(t, maxLogTextLen, utf8.RuneCountInString(entries[0].Message))
		sqlValue, ok := entries[0].ContextMap()["_sql"].(string)
		assert.True(t, ok)
		assert.Equal(t, maxLogTextLen, utf8.RuneCountInString(sqlValue))
	})

	t.Run("flow: keep bound trace field from logger.With", func(t *testing.T) {
		// Description: a logger that already has _trace_id should not be overwritten by defaults.
		// Expectation: the bound _trace_id value survives final output.
		logger, observed := newObservedLogger([]zap.Field{
			zap.String("_env", "test"),
			zap.String("_app", "unit-app"),
			zap.String("_app_ip", ""),
			zap.Int("_app_port", 0),
			zap.String("_trace_id", ""),
			zap.String("_span_id", ""),
		})
		logger.With(zap.String("_trace_id", "trace-abc")).Info("bound-trace")

		entries := observed.All()
		assert.Len(t, entries, 1)
		assert.Equal(t, "trace-abc", entries[0].ContextMap()["_trace_id"])
	})
}

func newObservedLogger(defaults []zap.Field) (*zap.Logger, *observer.ObservedLogs) {
	core, observed := observer.New(zap.InfoLevel)
	return zap.New(newRequiredFieldsCore(core, defaults)), observed
}
