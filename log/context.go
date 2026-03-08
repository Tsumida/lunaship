package log

import (
	"context"

	"go.uber.org/zap"
)

type contextKey int

const (
	fieldsKey contextKey = iota
	traceIDKey
	spanIDKey
	parentSpanIDKey
	sampledKey
)

func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	if len(fields) == 0 {
		return ctx
	}
	current, _ := ctx.Value(fieldsKey).([]zap.Field)
	merged := make([]zap.Field, 0, len(current)+len(fields))
	merged = append(merged, current...)
	merged = append(merged, fields...)
	return context.WithValue(ctx, fieldsKey, merged)
}

func FieldsFromContext(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	fields, _ := ctx.Value(fieldsKey).([]zap.Field)
	return fields
}

func WithTrace(ctx context.Context, traceID, spanID, parentSpanID string, sampled bool) context.Context {
	if traceID != "" {
		ctx = context.WithValue(ctx, traceIDKey, traceID)
		ctx = WithFields(ctx, zap.String("_trace_id", traceID))
	}
	if spanID != "" {
		ctx = context.WithValue(ctx, spanIDKey, spanID)
		ctx = WithFields(ctx, zap.String("_span_id", spanID))
	}
	if parentSpanID != "" {
		ctx = context.WithValue(ctx, parentSpanIDKey, parentSpanID)
		ctx = WithFields(ctx, zap.String("_parent_span_id", parentSpanID))
	}
	ctx = context.WithValue(ctx, sampledKey, sampled)
	return ctx
}

func TraceFromContext(ctx context.Context) (traceID, spanID string, ok bool) {
	if ctx == nil {
		return "", "", false
	}
	traceID, _ = ctx.Value(traceIDKey).(string)
	spanID, _ = ctx.Value(spanIDKey).(string)
	if traceID == "" && spanID == "" {
		return "", "", false
	}
	return traceID, spanID, true
}

func IsSampled(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	sampled, ok := ctx.Value(sampledKey).(bool)
	if !ok {
		return true
	}
	return sampled
}

func Logger(ctx context.Context) *zap.Logger {
	return GlobalLog().With(FieldsFromContext(ctx)...)
}
