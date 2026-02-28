package interceptor

import (
	"context"
	"net"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func NewTraceInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			span, ctx := startServerSpan(ctx, req)
			defer span.End()

			ctx = log.WithFields(ctx, requestBaseFields(req)...)

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
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header()))
	return span, ctx
}

func traceIdentifiers(sc trace.SpanContext) (traceID, spanID string, sampled bool) {
	if !sc.IsValid() {
		return "", "", true
	}
	return sc.TraceID().String(), sc.SpanID().String(), sc.TraceFlags().IsSampled()
}

func requestBaseFields(req connect.AnyRequest) []zap.Field {
	fields := make([]zap.Field, 0, 12)
	callerIP, callerPort := parseAddress(req.Peer().Addr)
	calleeApp, calleeIP, calleePort := log.AppIdentity()
	fields = append(fields,
		zap.String("_caller_ip", callerIP),
		zap.Int("_caller_port", callerPort),
		zap.String("_caller_app", ""),
		zap.String("_callee_ip", calleeIP),
		zap.Int("_callee_port", calleePort),
		zap.String("_callee_app", calleeApp),
		zap.String("_rpc_target", req.Spec().Procedure),
		zap.String("_rpc_protocol", req.Peer().Protocol),
		zap.String("_http_method", req.HTTPMethod()),
	)
	return fields
}

func parseAddress(addr string) (string, int) {
	if addr == "" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err == nil {
		port, parseErr := strconv.Atoi(strings.TrimSpace(portStr))
		if parseErr != nil {
			return host, 0
		}
		return host, port
	}
	lastColon := strings.LastIndex(addr, ":")
	if lastColon > 0 && lastColon < len(addr)-1 {
		port, parseErr := strconv.Atoi(strings.TrimSpace(addr[lastColon+1:]))
		if parseErr == nil {
			return strings.TrimSpace(addr[:lastColon]), port
		}
	}
	return strings.TrimSpace(addr), 0
}

func parentSpanIDFromContext(ctx context.Context) string {
	parent := trace.SpanContextFromContext(ctx)
	if !parent.IsValid() {
		return ""
	}
	return parent.SpanID().String()
}
