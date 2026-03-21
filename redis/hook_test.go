package redis

import (
	"context"
	"path/filepath"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap/zapcore"
)

func TestParseRedisAddr(t *testing.T) {
	t.Run("flow: host and port are parsed from ip:port", func(t *testing.T) {
		// Description: parser receives a normal redis endpoint with host and port.
		// Expectation: host and port are extracted correctly.
		host, port := parseRedisAddr("192.168.0.103:6379")
		assert.Equal(t, "192.168.0.103", host, "host should be parsed from endpoint")
		assert.Equal(t, 6379, port, "port should be parsed from endpoint")
	})

	t.Run("flow: invalid endpoint falls back to zero values", func(t *testing.T) {
		// Description: parser receives an invalid endpoint.
		// Expectation: parser should not panic and should return zero-like values.
		host, port := parseRedisAddr("invalid-endpoint")
		assert.Equal(t, "invalid-endpoint", host, "invalid endpoint should be kept as host fallback")
		assert.Equal(t, 0, port, "invalid endpoint should have zero port")
	})
}

func TestParseRedisAddrs(t *testing.T) {
	t.Run("flow: parser picks first usable endpoint", func(t *testing.T) {
		// Description: address list contains empty and valid endpoints.
		// Expectation: parser should return the first valid host and port pair.
		host, port := parseRedisAddrs([]string{"", "  ", "127.0.0.1:6379", "10.0.0.1:6379"})
		assert.Equal(t, "127.0.0.1", host, "first valid host should be selected")
		assert.Equal(t, 6379, port, "first valid port should be selected")
	})
}

func TestLuaSHAFromCommand(t *testing.T) {
	t.Run("flow: evalsha command exposes sha field", func(t *testing.T) {
		// Description: command is evalsha with sha argument in position 1.
		// Expectation: helper returns sha value for redis log field.
		cmd := redis.NewCmd(context.Background(), "EVALSHA", "abc123", "1", "k1", "v1")
		assert.Equal(t, "abc123", luaSHAFromCommand(cmd), "sha should be extracted from evalsha command")
	})

	t.Run("flow: non evalsha command has empty sha", func(t *testing.T) {
		// Description: command is not evalsha.
		// Expectation: helper should return empty string.
		cmd := redis.NewCmd(context.Background(), "GET", "k1")
		assert.Equal(t, "", luaSHAFromCommand(cmd), "non evalsha command should not expose sha")
	})
}

func TestRedisTraceLogHook_IgnoresRedisNilReply(t *testing.T) {
	t.Run("flow: redis.Nil is treated as normal nil reply (cache miss) for span status", func(t *testing.T) {
		// Description: redis GET returns redis.Nil to represent a nil reply (missing key).
		// Expectation: hook should not mark the span as error, but should still return redis.Nil to caller.
		log.InitLog(
			filepath.Join(t.TempDir(), "info.log"),
			filepath.Join(t.TempDir(), "warn.log"),
			zapcore.InfoLevel,
		)

		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
		otel.SetTracerProvider(tp)

		h := newRedisTraceLogHook([]string{"127.0.0.1:6379"})
		process := h.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
			return redis.Nil
		})

		cmd := redis.NewCmd(context.Background(), "GET", "missing-key")
		err := process(context.Background(), cmd)
		assert.ErrorIs(t, err, redis.Nil, "hook should not swallow redis.Nil")

		spans := recorder.Ended()
		assert.Len(t, spans, 1, "one span should be ended per command")
		assert.Equal(t, "redis.get", spans[0].Name(), "span name should follow command name")
		assert.NotEqual(t, codes.Error, spans[0].Status().Code, "span status should not be error on nil reply")
	})

	t.Run("flow: non-nil error is still treated as error for span status", func(t *testing.T) {
		// Description: redis command returns a real error (not redis.Nil).
		// Expectation: hook should mark the span status as error.
		log.InitLog(
			filepath.Join(t.TempDir(), "info.log"),
			filepath.Join(t.TempDir(), "warn.log"),
			zapcore.InfoLevel,
		)

		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
		otel.SetTracerProvider(tp)

		h := newRedisTraceLogHook([]string{"127.0.0.1:6379"})
		process := h.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
			return assert.AnError
		})

		cmd := redis.NewCmd(context.Background(), "GET", "any-key")
		err := process(context.Background(), cmd)
		assert.ErrorIs(t, err, assert.AnError, "hook should return original error")

		spans := recorder.Ended()
		assert.Len(t, spans, 1, "one span should be ended per command")
		assert.Equal(t, codes.Error, spans[0].Status().Code, "span status should be error on real error")
	})
}
