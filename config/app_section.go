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

func (c *AppSection) Normalize() {
	if c == nil {
		return
	}
	c.AppName = strings.TrimSpace(c.AppName)
	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	c.Trace.OTLPExporterOTLPEndpoint = strings.TrimSpace(c.Trace.OTLPExporterOTLPEndpoint)
	c.Trace.OTLPExporterOTLPProtocol = strings.ToLower(strings.TrimSpace(c.Trace.OTLPExporterOTLPProtocol))
	c.Trace.OTelResourceOTLPTraceEndpoint = strings.TrimSpace(c.Trace.OTelResourceOTLPTraceEndpoint)
	c.Pprof.Mode = strings.ToLower(strings.TrimSpace(c.Pprof.Mode))
}

func (c AppSection) Validate(problems *configparse.Problems) {
	if strings.TrimSpace(c.AppName) == "" {
		problems.Add("app.app_name", "is required")
	}

	level := strings.ToLower(strings.TrimSpace(c.Log.Level))
	if _, ok := validLogLevels[level]; !ok {
		problems.Add("app.log.level", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validLogLevels)))
	}

	if strings.ToLower(strings.TrimSpace(c.Trace.OTLPExporterOTLPProtocol)) != "http" {
		problems.Add("app.trace.otel_exporter_otlp_protocol", `must be "http" in v1`)
	}

	mode := strings.ToLower(strings.TrimSpace(c.Pprof.Mode))
	if _, ok := validPprofModes[mode]; !ok {
		problems.Add("app.pprof.mode", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validPprofModes)))
	}
}
