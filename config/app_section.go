package config

import (
	"fmt"
	"strings"

	"github.com/tsumida/lunaship/configparse"
)

const (
	defaultLogLevel      = "info"
	defaultTraceEnabled  = false
	defaultTraceProtocol = "http"
	defaultPprofEnabled  = true
	defaultPprofMode     = "dynamic"
)

var (
	validLogLevels = map[string]struct{}{
		"debug": {},
		"info":  {},
		"warn":  {},
		"error": {},
	}
	validPprofModes = map[string]struct{}{
		"server":  {},
		"dynamic": {},
	}
)

type AppSection struct {
	AppName string         `toml:"app_name"`
	Log     AppLogConfig   `toml:"log"`
	Trace   AppTraceConfig `toml:"trace"`
	Pprof   AppPprofConfig `toml:"pprof"`
}

type AppLogConfig struct {
	Level string `toml:"level"`
}

type AppTraceConfig struct {
	Enabled                       bool   `toml:"enabled"`
	OTLPExporterOTLPEndpoint      string `toml:"otel_exporter_otlp_endpoint"`
	OTLPExporterOTLPProtocol      string `toml:"otel_exporter_otlp_protocol"`
	OTelResourceOTLPTraceEndpoint string `toml:"otel_resource_otlp_trace_endpoint"`
}

type AppPprofConfig struct {
	Mode    string `toml:"mode"`
	Enabled bool   `toml:"enabled"`
}

func defaultAppSection() AppSection {
	return AppSection{
		Log: AppLogConfig{
			Level: defaultLogLevel,
		},
		Trace: AppTraceConfig{
			Enabled:                  defaultTraceEnabled,
			OTLPExporterOTLPProtocol: defaultTraceProtocol,
		},
		Pprof: AppPprofConfig{
			Mode:    defaultPprofMode,
			Enabled: defaultPprofEnabled,
		},
	}
}

func decodeAppSection(raw map[string]any, problems *configparse.Problems) AppSection {
	cfg := defaultAppSection()

	for key := range raw {
		switch key {
		case "app_name", "log", "trace", "pprof":
		default:
			problems.Add("app."+key, "unknown field")
		}
	}

	if value, ok := configparse.OptionalString(raw, "app_name", "app.app_name", problems); ok {
		cfg.AppName = strings.TrimSpace(value)
	}
	if strings.TrimSpace(cfg.AppName) == "" {
		problems.Add("app.app_name", "is required")
	}

	if table, ok := configparse.OptionalTable(raw, "log", "app.log", problems); ok {
		decodeAppLogSection(table, &cfg.Log, problems)
	}
	if table, ok := configparse.OptionalTable(raw, "trace", "app.trace", problems); ok {
		decodeAppTraceSection(table, &cfg.Trace, problems)
	}
	if table, ok := configparse.OptionalTable(raw, "pprof", "app.pprof", problems); ok {
		decodeAppPprofSection(table, &cfg.Pprof, problems)
	}

	return cfg
}

func decodeAppLogSection(raw map[string]any, cfg *AppLogConfig, problems *configparse.Problems) {
	for key := range raw {
		switch key {
		case "level":
		default:
			problems.Add("app.log."+key, "unknown field")
		}
	}

	if value, ok := configparse.OptionalString(raw, "level", "app.log.level", problems); ok {
		cfg.Level = strings.ToLower(strings.TrimSpace(value))
	}
	if _, ok := validLogLevels[cfg.Level]; !ok {
		problems.Add("app.log.level", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validLogLevels)))
	}
}

func decodeAppTraceSection(raw map[string]any, cfg *AppTraceConfig, problems *configparse.Problems) {
	for key := range raw {
		switch key {
		case "enabled", "otel_exporter_otlp_endpoint", "otel_exporter_otlp_protocol", "otel_resource_otlp_trace_endpoint":
		default:
			problems.Add("app.trace."+key, "unknown field")
		}
	}

	if value, ok := configparse.OptionalBool(raw, "enabled", "app.trace.enabled", problems); ok {
		cfg.Enabled = value
	}
	if value, ok := configparse.OptionalString(raw, "otel_exporter_otlp_endpoint", "app.trace.otel_exporter_otlp_endpoint", problems); ok {
		cfg.OTLPExporterOTLPEndpoint = strings.TrimSpace(value)
	}
	if value, ok := configparse.OptionalString(raw, "otel_exporter_otlp_protocol", "app.trace.otel_exporter_otlp_protocol", problems); ok {
		cfg.OTLPExporterOTLPProtocol = strings.ToLower(strings.TrimSpace(value))
	}
	if value, ok := configparse.OptionalString(raw, "otel_resource_otlp_trace_endpoint", "app.trace.otel_resource_otlp_trace_endpoint", problems); ok {
		cfg.OTelResourceOTLPTraceEndpoint = strings.TrimSpace(value)
	}

	if cfg.OTLPExporterOTLPProtocol != "http" {
		problems.Add("app.trace.otel_exporter_otlp_protocol", `must be "http" in v1`)
	}
}

func decodeAppPprofSection(raw map[string]any, cfg *AppPprofConfig, problems *configparse.Problems) {
	for key := range raw {
		switch key {
		case "mode", "enabled":
		default:
			problems.Add("app.pprof."+key, "unknown field")
		}
	}

	if value, ok := configparse.OptionalString(raw, "mode", "app.pprof.mode", problems); ok {
		cfg.Mode = strings.ToLower(strings.TrimSpace(value))
	}
	if value, ok := configparse.OptionalBool(raw, "enabled", "app.pprof.enabled", problems); ok {
		cfg.Enabled = value
	}

	if _, ok := validPprofModes[cfg.Mode]; !ok {
		problems.Add("app.pprof.mode", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validPprofModes)))
	}
}
