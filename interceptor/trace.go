package interceptor

import (
	"context"
	"net"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/log"
	"github.com/uber/jaeger-client-go"
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
			defer span.Finish()

			ctx = log.WithFields(ctx, requestBaseFields(req)...)

			resp, err := next(ctx, req)
			if err != nil {
				ext.Error.Set(span, true)
				span.LogFields(
					otlog.String("event", "error"),
					otlog.String("message", err.Error()),
				)
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
			span := startClientSpan(ctx, req)
			defer span.Finish()

			resp, err := next(ctx, req)
			if err != nil {
				ext.Error.Set(span, true)
				span.LogFields(
					otlog.String("event", "error"),
					otlog.String("message", err.Error()),
				)
			}
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func startServerSpan(ctx context.Context, req connect.AnyRequest) (opentracing.Span, context.Context) {
	tracer := opentracing.GlobalTracer()
	carrier := opentracing.HTTPHeadersCarrier(req.Header())
	parentCtx, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
	if err != nil && err != opentracing.ErrSpanContextNotFound && !infra.TraceErrorLogDisabled() {
		log.GlobalLog().Warn("trace extract failed", zap.Error(err))
	}

	spanName := req.Spec().Procedure
	span := tracer.StartSpan(spanName, ext.RPCServerOption(parentCtx))
	ext.SpanKindRPCServer.Set(span)
	ext.Component.Set(span, "connect")
	ext.HTTPMethod.Set(span, req.HTTPMethod())

	ctx = opentracing.ContextWithSpan(ctx, span)
	traceID, spanID, sampled := traceIdentifiers(span)
	ctx = log.WithTrace(ctx, traceID, spanID, sampled)
	return span, ctx
}

func startClientSpan(ctx context.Context, req connect.AnyRequest) opentracing.Span {
	tracer := opentracing.GlobalTracer()
	spanName := req.Spec().Procedure

	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan != nil {
		span := tracer.StartSpan(spanName, opentracing.ChildOf(parentSpan.Context()))
		ext.SpanKindRPCClient.Set(span)
		ext.Component.Set(span, "connect")
		if method := req.HTTPMethod(); method != "" {
			ext.HTTPMethod.Set(span, method)
		}
		injectTraceHeaders(tracer, span, req.Header())
		return span
	}

	span := tracer.StartSpan(spanName)
	ext.SpanKindRPCClient.Set(span)
	ext.Component.Set(span, "connect")
	if method := req.HTTPMethod(); method != "" {
		ext.HTTPMethod.Set(span, method)
	}
	injectTraceHeaders(tracer, span, req.Header())
	return span
}

func injectTraceHeaders(tracer opentracing.Tracer, span opentracing.Span, header http.Header) {
	if tracer == nil || span == nil {
		return
	}
	carrier := opentracing.HTTPHeadersCarrier(header)
	if err := tracer.Inject(span.Context(), opentracing.HTTPHeaders, carrier); err != nil {
		if !infra.TraceErrorLogDisabled() {
			log.GlobalLog().Warn("trace inject failed", zap.Error(err))
		}
	}
}

func traceIdentifiers(span opentracing.Span) (traceID, spanID string, sampled bool) {
	if span == nil {
		return "", "", true
	}
	sc, ok := span.Context().(jaeger.SpanContext)
	if !ok {
		return "", "", true
	}
	return sc.TraceID().String(), sc.SpanID().String(), sc.IsSampled()
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
