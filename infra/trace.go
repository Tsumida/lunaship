package infra

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	jaegerconfig "github.com/uber/jaeger-client-go/config"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

type TraceConfig struct {
	Enabled          bool
	ServiceName      string
	AgentHost        string
	AgentPort        string
	CollectorEndpoint string
	SamplerType      string
	SamplerRate      float64
	ErrorLogDisabled bool
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
		os.Getenv("JAEGER_SERVICE_NAME"),
		utils.StrOrDefault(os.Getenv("SERVICE_ID"), "lunaship"),
	)

	conf := TraceConfig{
		Enabled:          enabled,
		ServiceName:      serviceName,
		AgentHost:        utils.StrOrDefault(os.Getenv("JAEGER_AGENT_HOST"), "127.0.0.1"),
		AgentPort:        utils.StrOrDefault(os.Getenv("JAEGER_AGENT_PORT"), "6831"),
		CollectorEndpoint: os.Getenv("JAEGER_COLLECTOR_ENDPOINT"),
		SamplerType:      samplerType,
		SamplerRate:      rate,
		ErrorLogDisabled: errorLogDisabled,
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
		zap.String("collector_endpoint", conf.CollectorEndpoint),
		zap.String("agent_host", conf.AgentHost),
		zap.String("agent_port", conf.AgentPort),
		zap.Bool("error_log_disabled", conf.ErrorLogDisabled),
	)
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

	cfg := &jaegerconfig.Configuration{
		ServiceName: conf.ServiceName,
		Sampler: &jaegerconfig.SamplerConfig{
			Type:  conf.SamplerType,
			Param: conf.SamplerRate,
		},
		Reporter: &jaegerconfig.ReporterConfig{
			LogSpans: false,
		},
	}

	if conf.CollectorEndpoint != "" {
		cfg.Reporter.CollectorEndpoint = conf.CollectorEndpoint
	} else {
		cfg.Reporter.LocalAgentHostPort = net.JoinHostPort(conf.AgentHost, conf.AgentPort)
	}

	tracer, closer, err := cfg.NewTracer(jaegerconfig.Logger(&traceReporterLogger{}))
	if err != nil {
		return nil, err
	}
	opentracing.SetGlobalTracer(tracer)
	if closer != nil {
		traceCloser = closer.Close
	}
	return closer, nil
}

func CloseTracing() error {
	return traceCloser()
}

type traceReporterLogger struct{}

func (l *traceReporterLogger) Error(msg string) {
	traceReporterErrorCounter.Inc()
	if traceErrorLogDisabled {
		return
	}
	log.GlobalLog().Error("trace reporter error", zap.String("message", msg))
}

func (l *traceReporterLogger) Infof(msg string, args ...interface{}) {
	if traceErrorLogDisabled {
		return
	}
	log.GlobalLog().Info("trace reporter info", zap.String("message", fmt.Sprintf(msg, args...)))
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
