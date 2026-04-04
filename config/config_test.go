package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type customKeyValue struct {
	Key1        string   `toml:"key1"`
	Key2        string   `toml:"key2"`
	BlockedList []string `toml:"blocked_list"`
}

func TestLoadFromBytes(t *testing.T) {
	t.Run("flow: load complete phase 1 config with defaults and named instances", func(t *testing.T) {
		// Description: valid app.toml contains app metadata plus named redis/mysql/kafka instances.
		// Expectation: loader returns a populated AppConfig and applies v1 defaults for omitted fields.
		body := []byte(`
[app]
app_name = "logs-demo"

[redis.dev-redis]
addr = "redis.tinyinfra.dev"
port = 6379

[mysql.dev-mysql]
host = "mysql.tinyinfra.dev"
port = 3306
username = "root"
database = "test"

[kafka.producer.dev]
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"

[kafka.consumer.dev]
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"
consumer_group = "demo-consumer-group"

[custom-kv]
key1 = "value1"
key2 = "value2"
blocked_list = ["user1", "user2"]
`)

		cfg, err := LoadFromBytes(body)
		assert.NoError(t, err, "valid config should load successfully")
		if !assert.NotNil(t, cfg, "loader should return config on success") {
			return
		}

		assert.Equal(t, "logs-demo", cfg.App.AppName, "app name should come from app.app_name")
		assert.Equal(t, defaultLogLevel, cfg.App.Log.Level, "log level should default to info")
		assert.Equal(t, defaultTraceEnabled, cfg.App.Trace.Enabled, "trace enabled should default to false")
		assert.Equal(t, defaultTraceProtocol, cfg.App.Trace.OTLPExporterOTLPProtocol, "trace protocol should default to http")
		assert.Equal(t, defaultTraceSampleRate, cfg.App.Trace.SampleRate, "trace sample rate should default to 0.001")
		assert.Equal(t, defaultPprofMode, cfg.App.Pprof.Mode, "pprof mode should default to dynamic")
		assert.Equal(t, defaultPprofEnabled, cfg.App.Pprof.Enabled, "pprof enabled should default to true")

		redisCfg, ok := cfg.Redis.Instances["dev-redis"]
		assert.True(t, ok, "named redis instance should be available")
		assert.Equal(t, "redis.tinyinfra.dev", redisCfg.Addr, "redis addr should match TOML")
		assert.Equal(t, 6379, redisCfg.Port, "redis port should match TOML")
		assert.Equal(t, 0, redisCfg.DB, "redis db should default to zero value")

		assert.Equal(t, 200, cfg.MySQL.MaxOpenConns, "mysql max open conns should keep v1 default when omitted")
		assert.Equal(t, 100, cfg.MySQL.MaxIdleConns, "mysql max idle conns should keep v1 default when omitted")
		mysqlCfg, ok := cfg.MySQL.Instances["dev-mysql"]
		assert.True(t, ok, "named mysql instance should be available")
		assert.Equal(t, "mysql.tinyinfra.dev", mysqlCfg.Host, "mysql host should match TOML")
		assert.Equal(t, "root", mysqlCfg.Username, "mysql username should match TOML")
		assert.False(t, mysqlCfg.PingEnabled, "ping_enabled should default to false")

		producerCfg, ok := cfg.Kafka.Producer["dev"]
		assert.True(t, ok, "named kafka producer should be available")
		assert.Equal(t, []string{"kafka.tinyinfra.dev:9092"}, producerCfg.Brokers, "producer brokers should match TOML")
		assert.Equal(t, "all", producerCfg.Acks, "producer acks should default to all")

		consumerCfg, ok := cfg.Kafka.Consumer["dev"]
		assert.True(t, ok, "named kafka consumer should be available")
		assert.Equal(t, "demo-consumer-group", consumerCfg.ConsumerGroup, "consumer group should match TOML")

		var customCfg customKeyValue
		err = cfg.GetCustomConfig(".custom-kv", &customCfg)
		assert.NoError(t, err, "custom config should decode from retained raw section tree")
		assert.Equal(t, "value1", customCfg.Key1, "custom key1 should match TOML")
		assert.Equal(t, []string{"user1", "user2"}, customCfg.BlockedList, "custom list should match TOML")
	})

	t.Run("flow: trace sample_rate is parsed and validated", func(t *testing.T) {
		// Description: app config sets a trace sampling ratio to reduce exported spans.
		// Expectation: loader keeps the provided ratio and rejects values outside the supported precision/range.
		cfg, err := LoadFromBytes([]byte(`
[app]
app_name = "logs-demo"

[app.trace]
sample_rate = 0.125
`))
		assert.NoError(t, err, "valid sample rate should load successfully")
		if !assert.NotNil(t, cfg, "loader should return config for valid sample rate") {
			return
		}

		assert.Equal(t, 0.125, cfg.App.Trace.SampleRate, "sample rate should keep the configured ratio")

		_, err = LoadFromBytes([]byte(`
[app]
app_name = "logs-demo"

[app.trace]
sample_rate = 0.0009
`))
		var loadErr *LoadError
		assert.Error(t, err, "out-of-range sample rate should fail validation")
		assert.True(t, errors.As(err, &loadErr), "validation failure should unwrap to LoadError")
		if assert.NotNil(t, loadErr, "validation failure should produce LoadError") {
			assert.Contains(t, loadErr.Details, ErrorDetail{Path: "app.trace.sample_rate", Message: "must be between 0.001 and 1.0"}, "range violation should be reported")
		}

		_, err = LoadFromBytes([]byte(`
[app]
app_name = "logs-demo"

[app.trace]
sample_rate = 0.1234
`))
		assert.Error(t, err, "sample rate with too many decimal places should fail validation")
		assert.True(t, errors.As(err, &loadErr), "precision failure should unwrap to LoadError")
		if assert.NotNil(t, loadErr, "precision failure should produce LoadError") {
			assert.Contains(t, loadErr.Details, ErrorDetail{Path: "app.trace.sample_rate", Message: "must use at most 3 decimal places"}, "precision violation should be reported")
		}
	})

	t.Run("flow: malformed toml returns parse error", func(t *testing.T) {
		// Description: the input TOML is syntactically broken.
		// Expectation: loader returns a structured parse error instead of a validation error.
		_, err := LoadFromBytes([]byte(`
[app
app_name = "broken"
`))

		var loadErr *LoadError
		assert.Error(t, err, "malformed TOML should fail")
		assert.True(t, errors.As(err, &loadErr), "error should unwrap to LoadError")
		if assert.NotNil(t, loadErr, "parse failure should produce LoadError") {
			assert.Equal(t, ErrorKindParse, loadErr.Kind, "kind should be parse")
			assert.Empty(t, loadErr.Details, "parse errors should not use validation details")
		}
	})

	t.Run("flow: missing required fields returns validation details", func(t *testing.T) {
		// Description: config omits required app name and mysql database fields.
		// Expectation: validation error exposes the exact failing paths.
		_, err := LoadFromBytes([]byte(`
[app]

[mysql.default]
host = "mysql.tinyinfra.dev"
port = 3306
username = "root"
`))

		var loadErr *LoadError
		assert.Error(t, err, "invalid config should fail validation")
		assert.True(t, errors.As(err, &loadErr), "error should unwrap to LoadError")
		if assert.NotNil(t, loadErr, "validation failure should produce LoadError") {
			assert.Equal(t, ErrorKindValidation, loadErr.Kind, "kind should be validation")
			assert.Contains(t, loadErr.Details, ErrorDetail{Path: "app.app_name", Message: "is required"}, "missing app name should be reported")
			assert.Contains(t, loadErr.Details, ErrorDetail{Path: "mysql.default.database", Message: "is required"}, "missing mysql database should be reported")
		}
	})

	t.Run("flow: missing custom section returns lookup error", func(t *testing.T) {
		// Description: custom config lookup asks for a section that is absent from app.toml.
		// Expectation: GetCustomConfig returns a clear not-found error.
		cfg, err := LoadFromBytes([]byte(`
[app]
app_name = "logs-demo"
`))
		assert.NoError(t, err, "base config should still load without custom sections")

		var customCfg customKeyValue
		err = cfg.GetCustomConfig(".custom-kv", &customCfg)
		assert.Error(t, err, "missing custom section should fail lookup")
		assert.Contains(t, err.Error(), `custom config section ".custom-kv" not found`, "error should identify the missing section")
	})
}

func TestGlobal(t *testing.T) {
	t.Run("flow: set and get global app config", func(t *testing.T) {
		// Description: bootstrap publishes a loaded AppConfig for later reuse.
		// Expectation: Global should return the same pointer until explicitly cleared or replaced.
		previous := Global()
		t.Cleanup(func() {
			SetGlobal(previous)
		})

		cfg := &AppConfig{
			App: AppSection{
				AppName: "logs-demo",
			},
		}

		SetGlobal(cfg)

		assert.Same(t, cfg, Global(), "global config should return the pointer published by bootstrap")
	})

	t.Run("flow: global app config can be cleared", func(t *testing.T) {
		// Description: tests or alternate bootstrap flows need to reset the shared config state.
		// Expectation: SetGlobal(nil) should clear the shared pointer safely.
		previous := Global()
		t.Cleanup(func() {
			SetGlobal(previous)
		})

		SetGlobal(nil)

		assert.Nil(t, Global(), "cleared global config should return nil")
	})
}
