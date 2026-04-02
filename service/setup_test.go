package service

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/config"
)

func TestSelectBootstrapInstance(t *testing.T) {
	t.Run("flow: default instance is preferred when multiple instances exist", func(t *testing.T) {
		// Description: bootstrap receives a default instance plus an additional named instance.
		// Expectation: svc.Run should pick the default instance to preserve current global client behavior.
		name, instance, ok, err := selectBootstrapInstance(map[string]int{
			"default": 1,
			"report":  2,
		}, "mysql")

		assert.NoError(t, err, "default instance selection should not fail")
		assert.True(t, ok, "selection should report an instance")
		assert.Equal(t, "default", name, "default instance should win over other names")
		assert.Equal(t, 1, instance, "selected instance should come from the default entry")
	})

	t.Run("flow: single named instance is accepted without default alias", func(t *testing.T) {
		// Description: bootstrap receives exactly one configured instance with a non-default name.
		// Expectation: svc.Run should use that sole instance because there is no ambiguity.
		name, instance, ok, err := selectBootstrapInstance(map[string]string{
			"dev-mysql": "dsn",
		}, "mysql")

		assert.NoError(t, err, "single instance should not require a default alias")
		assert.True(t, ok, "selection should report an instance")
		assert.Equal(t, "dev-mysql", name, "the only configured instance should be selected")
		assert.Equal(t, "dsn", instance, "the selected value should match the only configured instance")
	})

	t.Run("flow: multiple non-default instances fail fast with a clear error", func(t *testing.T) {
		// Description: bootstrap receives multiple named instances but no default alias.
		// Expectation: svc.Run should refuse to pick one implicitly because the current runtime only exposes one global client.
		_, _, ok, err := selectBootstrapInstance(map[string]struct{}{
			"analytics": {},
			"report":    {},
		}, "redis")

		assert.Error(t, err, "ambiguous bootstrap selection should fail")
		assert.False(t, ok, "selection should report no instance on ambiguity")
		assert.Contains(t, err.Error(), `svc.Run currently requires a "default" instance`, "error should explain the current bootstrap constraint")
		assert.Contains(t, err.Error(), "analytics, report", "error should list the configured instance names deterministically")
	})

	t.Run("flow: empty instance map is treated as module not configured", func(t *testing.T) {
		// Description: the app.toml omits a component section entirely.
		// Expectation: svc.Run should skip initialization without error.
		name, _, ok, err := selectBootstrapInstance(map[string]struct{}{}, "redis")

		assert.NoError(t, err, "omitted component should not fail bootstrap")
		assert.False(t, ok, "selection should report no instance when module is absent")
		assert.Empty(t, name, "absent module should not return an instance name")
	})
}

func TestTraceConfigFromAppConfig(t *testing.T) {
	t.Run("flow: dedicated traces endpoint wins over exporter endpoint", func(t *testing.T) {
		// Description: app config provides both a shared exporter endpoint and a traces-specific endpoint.
		// Expectation: svc.Run should use the traces endpoint and mark it as a traces-specific URL.
		cfg := &config.AppConfig{
			App: config.AppSection{
				AppName: "logs-demo",
				Trace: config.AppTraceConfig{
					Enabled:                       true,
					OTLPExporterOTLPEndpoint:      "otel.example:4318",
					OTLPExporterOTLPProtocol:      "http",
					OTelResourceOTLPTraceEndpoint: "http://collector.example:4318/v1/traces",
				},
			},
		}

		traceCfg := traceConfigFromAppConfig(cfg)
		assert.True(t, traceCfg.Enabled, "trace config should keep the enabled flag")
		assert.Equal(t, "logs-demo", traceCfg.ServiceName, "trace service name should come from app.app_name")
		assert.Equal(t, "http://collector.example:4318/v1/traces", traceCfg.OTLPEndpoint, "traces endpoint should win when configured")
		assert.True(t, traceCfg.OTLPTracesEndpoint, "dedicated traces endpoint should be marked explicitly")
		assert.Equal(t, "http/protobuf", traceCfg.OTLPProtocol, "http should be normalized to the SDK protocol value")
	})

	t.Run("flow: exporter endpoint is used when traces endpoint is absent", func(t *testing.T) {
		// Description: app config only provides the shared exporter endpoint.
		// Expectation: svc.Run should keep using the shared endpoint and let the trace layer append /v1/traces.
		cfg := &config.AppConfig{
			App: config.AppSection{
				AppName: "logs-demo",
				Trace: config.AppTraceConfig{
					Enabled:                  true,
					OTLPExporterOTLPEndpoint: "otel.example:4318",
				},
			},
		}

		traceCfg := traceConfigFromAppConfig(cfg)
		assert.Equal(t, "otel.example:4318", traceCfg.OTLPEndpoint, "shared exporter endpoint should be used when no traces URL is configured")
		assert.False(t, traceCfg.OTLPTracesEndpoint, "shared exporter endpoint should not be treated as a traces-specific URL")
		assert.Equal(t, "http/protobuf", traceCfg.OTLPProtocol, "empty protocol should fall back to the SDK default")
	})
}

func TestApplyRuntimeMetadata(t *testing.T) {
	t.Run("flow: app config sets runtime identity but preserves explicit service override", func(t *testing.T) {
		// Description: bootstrap loads app.app_name while SERVICE_ID is already injected by deployment.
		// Expectation: APP_NAME should always follow config, while SERVICE_ID keeps the explicit override.
		t.Setenv("SERVICE_ID", "metrics-service")
		cfg := &config.AppConfig{
			App: config.AppSection{
				AppName: "logs-demo",
			},
		}

		applyRuntimeMetadata(cfg)

		assert.Equal(t, "logs-demo", os.Getenv("APP_NAME"), "app name should always come from app config")
		assert.Equal(t, "metrics-service", os.Getenv("SERVICE_ID"), "explicit service id override should be preserved")
		assert.Equal(t, "logs-demo", os.Getenv("OTEL_SERVICE_NAME"), "otel service name should default to app name when unset")
	})
}
