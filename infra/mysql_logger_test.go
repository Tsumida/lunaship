package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	gormMySQL "gorm.io/driver/mysql"
	gormLogger "gorm.io/gorm/logger"
)

func TestMySQLGormLoggerTrace(t *testing.T) {
	t.Run("flow: successful query logs mysql metadata as info", func(t *testing.T) {
		// Description: successful SQL execution should produce an info log with mysql-specific fields.
		// Expectation: log contains _instance_ip/_instance_port/_database/_dur_ms/_sql and _is_slow_query=false.
		logger, observed := newObservedMySQLLogger(t)
		logger.slowThreshold = 500 * time.Millisecond

		logger.Trace(
			context.Background(),
			time.Now().Add(-120*time.Millisecond),
			func() (string, int64) { return "SELECT 1", 1 },
			nil,
		)

		entries := observed.All()
		assert.Len(t, entries, 1, "Trace should emit one info log for successful query")
		entry := entries[0]
		assert.Equal(t, zap.InfoLevel, entry.Level, "successful query should be info")
		assert.Equal(t, "SQL", entry.Message, "mysql trace message should be SQL")

		fields := entry.ContextMap()
		assert.Equal(t, "192.168.0.104", fields["_instance_ip"], "instance ip should be parsed from DSN")
		assert.EqualValues(t, 3306, fields["_instance_port"], "instance port should be parsed from DSN")
		assert.Equal(t, "example_db", fields["_database"], "database should be parsed from DSN")
		assert.Equal(t, "SELECT 1", fields["_sql"], "sql should be attached in log field")
		assert.False(t, fields["_is_slow_query"].(bool), "short query should not be marked as slow")
		assert.GreaterOrEqual(t, fields["_dur_ms"].(int64), int64(0), "duration should be non-negative")
	})

	t.Run("flow: slow query is marked and logged as warn", func(t *testing.T) {
		// Description: query duration exceeds configured threshold.
		// Expectation: logger emits warn level and sets _is_slow_query=true.
		logger, observed := newObservedMySQLLogger(t)
		logger.slowThreshold = 100 * time.Millisecond

		logger.Trace(
			context.Background(),
			time.Now().Add(-600*time.Millisecond),
			func() (string, int64) { return "SELECT sleep(1)", 1 },
			nil,
		)

		entries := observed.All()
		assert.Len(t, entries, 1, "Trace should emit one warn log for slow query")
		entry := entries[0]
		assert.Equal(t, zap.WarnLevel, entry.Level, "slow query should be warn")
		fields := entry.ContextMap()
		assert.True(t, fields["_is_slow_query"].(bool), "slow query should be marked")
		assert.Equal(t, "SELECT sleep(1)", fields["_sql"], "slow query sql should be logged")
	})

	t.Run("flow: query error is logged as error with error field", func(t *testing.T) {
		// Description: SQL execution returns an error.
		// Expectation: logger emits error level and contains _error field.
		logger, observed := newObservedMySQLLogger(t)

		logger.Trace(
			context.Background(),
			time.Now().Add(-20*time.Millisecond),
			func() (string, int64) { return "INSERT INTO users(id) VALUES(1)", 0 },
			errors.New("duplicate key"),
		)

		entries := observed.All()
		assert.Len(t, entries, 1, "Trace should emit one error log for failed query")
		entry := entries[0]
		assert.Equal(t, zap.ErrorLevel, entry.Level, "error query should be error")
		fields := entry.ContextMap()
		assert.Equal(t, "INSERT INTO users(id) VALUES(1)", fields["_sql"], "failed sql should be logged")
		assert.Contains(t, fields, "_error", "error log should contain _error field")
	})

	t.Run("flow: err record not found is not treated as error", func(t *testing.T) {
		// Description: gorm ErrRecordNotFound should not be considered as hard SQL error.
		// Expectation: logger emits info log and keeps mysql metadata fields.
		logger, observed := newObservedMySQLLogger(t)

		logger.Trace(
			context.Background(),
			time.Now().Add(-20*time.Millisecond),
			func() (string, int64) { return "SELECT * FROM users WHERE id=9999", 0 },
			gormLogger.ErrRecordNotFound,
		)

		entries := observed.All()
		assert.Len(t, entries, 1, "record-not-found should still emit one SQL log")
		entry := entries[0]
		assert.Equal(t, zap.InfoLevel, entry.Level, "record-not-found should be info, not error")
		fields := entry.ContextMap()
		assert.Equal(t, "SELECT * FROM users WHERE id=9999", fields["_sql"], "sql should be logged")
		assert.False(t, fields["_is_slow_query"].(bool), "short query should not be marked as slow")
		assert.NotContains(t, fields, "_error", "record-not-found should not log _error")
	})

	t.Run("flow: error takes precedence over slow query", func(t *testing.T) {
		// Description: query is both slow and failed.
		// Expectation: logger emits error level and still keeps _is_slow_query=true.
		logger, observed := newObservedMySQLLogger(t)
		logger.slowThreshold = 100 * time.Millisecond

		logger.Trace(
			context.Background(),
			time.Now().Add(-500*time.Millisecond),
			func() (string, int64) { return "UPDATE users SET name='x' WHERE id=1", 1 },
			errors.New("deadlock found"),
		)

		entries := observed.All()
		assert.Len(t, entries, 1, "failed slow query should emit one SQL log")
		entry := entries[0]
		assert.Equal(t, zap.ErrorLevel, entry.Level, "error should win over warn for slow query")
		fields := entry.ContextMap()
		assert.True(t, fields["_is_slow_query"].(bool), "slow marker should be preserved")
		assert.Contains(t, fields, "_error", "error field should be present")
	})

	t.Run("flow: silent mode suppresses trace logs", func(t *testing.T) {
		// Description: logger log level is switched to silent.
		// Expectation: Trace emits no log entry.
		logger, observed := newObservedMySQLLogger(t)
		silent, ok := logger.LogMode(gormLogger.Silent).(*mysqlGormLogger)
		assert.True(t, ok, "LogMode should keep mysql logger type")

		silent.Trace(
			context.Background(),
			time.Now().Add(-100*time.Millisecond),
			func() (string, int64) { return "SELECT 1", 1 },
			nil,
		)

		assert.Len(t, observed.All(), 0, "silent mode should not emit logs")
	})
}

func TestMySQLEndpointParse(t *testing.T) {
	t.Run("flow: valid dsn returns host port and database", func(t *testing.T) {
		// Description: parse mysql DSN with tcp host:port and database.
		// Expectation: parser extracts all fields correctly.
		host, port, db := mysqlEndpoint(
			"user:pwd@tcp(192.168.0.104:3306)/example_db?charset=utf8mb4&parseTime=True&loc=Local",
		)
		assert.Equal(t, "192.168.0.104", host, "host should match DSN address")
		assert.Equal(t, 3306, port, "port should match DSN address")
		assert.Equal(t, "example_db", db, "database should match DSN path")
	})

	t.Run("flow: invalid dsn falls back to zero values", func(t *testing.T) {
		// Description: parser receives malformed DSN.
		// Expectation: parser returns empty host/database and zero port without panic.
		host, port, db := mysqlEndpoint("bad-dsn")
		assert.Equal(t, "", host, "invalid dsn should return empty host")
		assert.Equal(t, 0, port, "invalid dsn should return zero port")
		assert.Equal(t, "", db, "invalid dsn should return empty database")
	})
}

func TestMySQLGormLoggerTraceSpan(t *testing.T) {
	t.Run("flow: trace emits mysql span without sql details", func(t *testing.T) {
		// Description: SQL trace should create one mysql child span with whitelisted attributes only.
		// Expectation: span contains target_type/remote.ip/remote.port/error.flag/duration.ms and no sql attributes.
		logger, _ := newObservedMySQLLogger(t)
		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider()
		tp.RegisterSpanProcessor(recorder)
		prevProvider := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		defer otel.SetTracerProvider(prevProvider)

		ctx, parent := otel.Tracer("test").Start(context.Background(), "parent")
		logger.Trace(
			ctx,
			time.Now().Add(-120*time.Millisecond),
			func() (string, int64) { return "SELECT * FROM users", 1 },
			nil,
		)
		parent.End()

		spans := recorder.Ended()
		mysqlSpan := findSpanByName(spans, "mysql")
		if assert.NotNil(t, mysqlSpan, "mysql span should be emitted") {
			attrs := attrsToMap(mysqlSpan.Attributes())
			assert.Equal(t, "mysql", attrs["target_type"], "target type should be mysql")
			assert.Equal(t, "192.168.0.104", attrs["remote.ip"], "remote ip should match DSN")
			assert.EqualValues(t, int64(3306), attrs["remote.port"], "remote port should match DSN")
			assert.Equal(t, false, attrs["error.flag"], "error flag should be false for successful query")
			assert.Contains(t, attrs, "duration.ms", "duration attribute should exist")
			assert.NotContains(t, attrs, "db.statement", "span should not leak sql statement")
			assert.NotContains(t, attrs, "_sql", "span should not expose structured sql field")
		}
	})

	t.Run("flow: trace marks span error for failed sql", func(t *testing.T) {
		// Description: failed SQL trace should mark error flag in span attributes.
		// Expectation: mysql span has error.flag=true.
		logger, _ := newObservedMySQLLogger(t)
		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider()
		tp.RegisterSpanProcessor(recorder)
		prevProvider := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		defer otel.SetTracerProvider(prevProvider)

		ctx, parent := otel.Tracer("test").Start(context.Background(), "parent")
		logger.Trace(
			ctx,
			time.Now().Add(-200*time.Millisecond),
			func() (string, int64) { return "UPDATE users SET name='x'", 1 },
			errors.New("deadlock"),
		)
		parent.End()

		spans := recorder.Ended()
		mysqlSpan := findSpanByName(spans, "mysql")
		if assert.NotNil(t, mysqlSpan, "mysql span should be emitted") {
			attrs := attrsToMap(mysqlSpan.Attributes())
			assert.Equal(t, true, attrs["error.flag"], "error flag should be true for failed query")
		}
	})
}

func newObservedMySQLLogger(t *testing.T) (*mysqlGormLogger, *observer.ObservedLogs) {
	t.Helper()
	core, observed := observer.New(zap.DebugLevel)
	base := NewMySQLGormLogger(gormMySQL.Config{
		DSN: "user:pwd@tcp(192.168.0.104:3306)/example_db?charset=utf8mb4&parseTime=True&loc=Local",
	})
	logger, ok := base.(*mysqlGormLogger)
	assert.True(t, ok, "NewMySQLGormLogger should return mysql logger implementation")
	logger.loggerFn = func(context.Context) *zap.Logger {
		return zap.New(core)
	}
	return logger, observed
}

func findSpanByName(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	return nil
}

func attrsToMap(attrs []attribute.KeyValue) map[string]any {
	values := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		values[string(attr.Key)] = attr.Value.AsInterface()
	}
	return values
}
