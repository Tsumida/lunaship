package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type TraceConfig struct {
	Enabled                bool
	ServiceName            string
	OTLPEndpoint           string
	OTLPProtocol           string
	OTLPTracesEndpoint     bool
	LegacyCollectorEndpoint bool
	SamplerType            string
	SamplerRate            float64
	ErrorLogDisabled       bool
}

var traceCloser = func() error { return nil }
var traceErrorLogDisabled bool
var traceReporterErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "lunaship_trace_reporter_errors_total",
	Help: "Total number of trace reporter errors",
})

func init() {
	prometheus.MustRegister(traceReporterErrorCounter)
}

func LoadTraceConfigFromEnv() (TraceConfig, error) {
	var parseErr error

	enabled, err := parseBoolEnv("TRACE_ENABLED", true)
	if err != nil {
		parseErr = err
	}
	errorLogDisabled, err := parseBoolEnv("TRACE_ERROR_LOG_DISABLED", false)
	if err != nil && parseErr == nil {
		parseErr = err
	}
	traceErrorLogDisabled = errorLogDisabled

	rate, err := parseFloatEnv("JAEGER_SAMPLER_RATE", 1.0)
	if err != nil && parseErr == nil {
		parseErr = err
	}
	samplerType := strings.TrimSpace(os.Getenv("JAEGER_SAMPLER_TYPE"))
	if samplerType == "" {
		samplerType = "const"
	}
	if samplerType == "const" {
		if rate == 0 {
			rate = 0
		} else {
			rate = 1
		}
	}

	serviceName := utils.StrOrDefault(
		os.Getenv("OTEL_SERVICE_NAME"),
		utils.StrOrDefault(
			os.Getenv("JAEGER_SERVICE_NAME"),
			utils.StrOrDefault(os.Getenv("SERVICE_ID"), "lunaship"),
		),
	)

	otlpProtocol := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	if otlpProtocol == "" {
		otlpProtocol = "http/protobuf"
	}

	var (
		otlpEndpoint            string
		otlpTracesEndpoint      bool
		legacyCollectorEndpoint bool
	)
	if tracesEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); tracesEndpoint != "" {
		otlpEndpoint = tracesEndpoint
		otlpTracesEndpoint = true
	} else if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
		otlpEndpoint = endpoint
	} else if legacy := strings.TrimSpace(os.Getenv("JAEGER_COLLECTOR_ENDPOINT")); legacy != "" {
		otlpEndpoint = legacy
		otlpTracesEndpoint = true
		legacyCollectorEndpoint = true
	}

	conf := TraceConfig{
		Enabled:                enabled,
		ServiceName:            serviceName,
		OTLPEndpoint:           otlpEndpoint,
		OTLPProtocol:           otlpProtocol,
		OTLPTracesEndpoint:     otlpTracesEndpoint,
		LegacyCollectorEndpoint: legacyCollectorEndpoint,
		SamplerType:            samplerType,
		SamplerRate:            rate,
		ErrorLogDisabled:       errorLogDisabled,
	}

	return conf, parseErr
}

func TraceErrorLogDisabled() bool {
	return traceErrorLogDisabled
}

func InitTracingFromEnv() error {
	conf, err := LoadTraceConfigFromEnv()
	if err != nil && !conf.ErrorLogDisabled {
		log.GlobalLog().Error("trace env parse failed", zap.Error(err))
	}
	log.GlobalLog().Info("trace config",
		zap.Bool("enabled", conf.Enabled),
		zap.String("service_name", conf.ServiceName),
		zap.String("sampler_type", conf.SamplerType),
		zap.Float64("sampler_rate", conf.SamplerRate),
		zap.String("otlp_endpoint", conf.OTLPEndpoint),
		zap.String("otlp_protocol", conf.OTLPProtocol),
		zap.Bool("otlp_traces_endpoint", conf.OTLPTracesEndpoint),
		zap.Bool("legacy_collector_endpoint", conf.LegacyCollectorEndpoint),
		zap.Bool("error_log_disabled", conf.ErrorLogDisabled),
	)
	if conf.LegacyCollectorEndpoint && !conf.ErrorLogDisabled {
		log.GlobalLog().Warn("legacy JAEGER_COLLECTOR_ENDPOINT detected; use OTEL_EXPORTER_OTLP_ENDPOINT instead")
	}
	if !conf.Enabled {
		return nil
	}
	closer, initErr := InitTracing(conf)
	if initErr != nil {
		if !conf.ErrorLogDisabled {
			log.GlobalLog().Error("trace init failed", zap.Error(initErr))
		}
		return initErr
	}
	if closer != nil {
		traceCloser = closer.Close
	}
	return nil
}

func InitTracing(conf TraceConfig) (io.Closer, error) {
	if !conf.Enabled {
		return nil, nil
	}
	traceErrorLogDisabled = conf.ErrorLogDisabled
	otel.SetErrorHandler(&traceErrorHandler{})

	if conf.OTLPProtocol != "" && conf.OTLPProtocol != "http/protobuf" {
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", conf.OTLPProtocol)
	}

	opts, err := otlpHTTPOptions(conf)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		context.Background(),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", conf.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(buildSampler(conf)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	closer := traceCloserFunc{shutdown: tp.Shutdown}
	traceCloser = closer.Close
	return closer, nil
}

func CloseTracing() error {
	return traceCloser()
}

type traceErrorHandler struct{}

func (h *traceErrorHandler) Handle(err error) {
	traceReporterErrorCounter.Inc()
	if traceErrorLogDisabled {
		return
	}
	if err == nil {
		return
	}
	log.GlobalLog().Error("trace reporter error", zap.Error(err))
}

type traceCloserFunc struct {
	shutdown func(context.Context) error
}

func (c traceCloserFunc) Close() error {
	if c.shutdown == nil {
		return nil
	}
	return c.shutdown(context.Background())
}

func buildSampler(conf TraceConfig) sdktrace.Sampler {
	samplerType := strings.ToLower(strings.TrimSpace(conf.SamplerType))
	switch samplerType {
	case "const":
		if conf.SamplerRate == 0 {
			return sdktrace.NeverSample()
		}
		return sdktrace.AlwaysSample()
	case "probabilistic", "rate", "ratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(clampSamplerRate(conf.SamplerRate)))
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(clampSamplerRate(conf.SamplerRate)))
	}
}

func clampSamplerRate(rate float64) float64 {
	if rate < 0 {
		return 0
	}
	if rate > 1 {
		return 1
	}
	return rate
}

func otlpHTTPOptions(conf TraceConfig) ([]otlptracehttp.Option, error) {
	endpoint := strings.TrimSpace(conf.OTLPEndpoint)
	if endpoint == "" {
		return []otlptracehttp.Option{
			otlptracehttp.WithEndpoint("127.0.0.1:4318"),
			otlptracehttp.WithURLPath("/v1/traces"),
			otlptracehttp.WithInsecure(),
		}, nil
	}

	raw := endpoint
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid OTLP endpoint: %q", endpoint)
	}

	path := strings.TrimRight(parsed.Path, "/")
	if conf.OTLPTracesEndpoint {
		if path == "" {
			path = "/v1/traces"
		}
	} else {
		if path == "" {
			path = "/v1/traces"
		} else {
			path = path + "/v1/traces"
		}
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(parsed.Host),
		otlptracehttp.WithURLPath(path),
	}
	if parsed.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	} else if parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported OTLP endpoint scheme: %s", parsed.Scheme)
	}

	return opts, nil
}

func parseBoolEnv(key string, defaultValue bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue, errors.New(key + " must be true/false")
	}
	return parsed, nil
}

func parseFloatEnv(key string, defaultValue float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue, errors.New(key + " must be a float")
	}
	return parsed, nil
}
