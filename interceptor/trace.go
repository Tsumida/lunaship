package interceptor

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const remoteServiceHeader = "X-Lunaship-Remote-Service"

func NewTraceInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			span, ctx := startServerSpan(ctx, req)
			defer span.End()

			resp, err := next(ctx, req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func NewTraceClientInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			span, ctx := startClientSpan(ctx, req)
			defer span.End()

			resp, err := next(ctx, req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func startServerSpan(ctx context.Context, req connect.AnyRequest) (trace.Span, context.Context) {
	tracer := otel.Tracer("lunaship/trace")
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header()))
	parentSpanID := parentSpanIDFromContext(ctx)

	spanName := req.Spec().Procedure
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("rpc.system", "connect"),
	)
	if method := req.HTTPMethod(); method != "" {
		span.SetAttributes(attribute.String("http.method", method))
	}

	traceID, spanID, sampled := traceIdentifiers(span.SpanContext())
	ctx = log.WithTrace(ctx, traceID, spanID, parentSpanID, sampled)
	return span, ctx
}

func startClientSpan(ctx context.Context, req connect.AnyRequest) (trace.Span, context.Context) {
	tracer := otel.Tracer("lunaship/trace")
	spanName := req.Spec().Procedure
	parentSpanID := parentSpanIDFromContext(ctx)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String("rpc.system", "connect"),
	)
	if method := req.HTTPMethod(); method != "" {
		span.SetAttributes(attribute.String("http.method", method))
	}
	traceID, spanID, sampled := traceIdentifiers(span.SpanContext())
	ctx = log.WithTrace(ctx, traceID, spanID, parentSpanID, sampled)
	if callerApp, _, _ := log.AppIdentity(); callerApp != "" {
		req.Header().Set(remoteServiceHeader, callerApp)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header()))
	return span, ctx
}

func traceIdentifiers(sc trace.SpanContext) (traceID, spanID string, sampled bool) {
	if !sc.IsValid() {
		return "", "", true
	}
	return sc.TraceID().String(), sc.SpanID().String(), sc.TraceFlags().IsSampled()
}

func parentSpanIDFromContext(ctx context.Context) string {
	parent := trace.SpanContextFromContext(ctx)
	if !parent.IsValid() {
		return ""
	}
	return parent.SpanID().String()
}

func remoteServiceFromHeader(raw string) string {
	return strings.TrimSpace(raw)
}
