package interceptor

import (
	"context"
	"net"
	"net/http"
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

var (
	loggedHeaderKeys = []string{
		"x-request-id",
		"x-device-ip",
		"x-forwarded-for",
		"x-real-ip",
		"user-agent",
		"content-type",
		"traceparent",
		"uber-trace-id",
	}
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

	spanName := req.Spec().Procedure
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("rpc.system", "connect"),
	)
	if method := req.HTTPMethod(); method != "" {
		span.SetAttributes(attribute.String("http.method", method))
	}

	traceID, spanID, sampled := traceIdentifiers(span.SpanContext())
	ctx = log.WithTrace(ctx, traceID, spanID, sampled)
	return span, ctx
}

func startClientSpan(ctx context.Context, req connect.AnyRequest) (trace.Span, context.Context) {
	tracer := otel.Tracer("lunaship/trace")
	spanName := req.Spec().Procedure

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String("rpc.system", "connect"),
	)
	if method := req.HTTPMethod(); method != "" {
		span.SetAttributes(attribute.String("http.method", method))
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

func requestBaseFields(req connect.AnyRequest) []zap.Field {
	fields := make([]zap.Field, 0, 12)
	if deviceIP := hostOnly(req.Peer().Addr); deviceIP != "" {
		fields = append(fields, zap.String("device_ip", deviceIP))
	}

	serverHost, serverPort := serverHostPort(req.Header())
	if serverHost != "" {
		fields = append(fields, zap.String("server_ip", serverHost))
	}
	if serverPort != "" {
		fields = append(fields, zap.String("server_port", serverPort))
	}
	fields = append(fields,
		zap.String("server_endpoint", req.Spec().Procedure),
		zap.String("http_method", req.HTTPMethod()),
	)

	fields = append(fields, headerFields(req.Header())...)
	return fields
}

func hostOnly(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	return addr
}

func serverHostPort(header http.Header) (string, string) {
	hostPort := strings.TrimSpace(header.Get("X-Forwarded-Host"))
	if hostPort == "" {
		hostPort = strings.TrimSpace(header.Get("Host"))
	}
	if hostPort == "" {
		return "", ""
	}
	if strings.Contains(hostPort, ",") {
		parts := strings.Split(hostPort, ",")
		hostPort = strings.TrimSpace(parts[0])
	}
	host, port, err := net.SplitHostPort(hostPort)
	if err == nil {
		return host, port
	}
	return hostPort, ""
}

func headerFields(header http.Header) []zap.Field {
	fields := make([]zap.Field, 0, len(loggedHeaderKeys))
	for _, key := range loggedHeaderKeys {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		fields = append(fields, zap.String(normalizeHeaderKey(key), value))
	}
	return fields
}

func normalizeHeaderKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "-", "_")
	return "header_" + key
}
