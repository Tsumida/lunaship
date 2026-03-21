package tests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/redis"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap/zapcore"
)

func initEnv() {
	// ENV BIND_ADDR=""
	// ENV ERR_FILE=""
	// ENV JAEGER_AGENT_HOST=""
	// ENV JAEGER_AGENT_PORT=""
	// ENV JAEGER_COLLECTOR_ENDPOINT=""
	// ENV JAEGER_SERVICE_NAME=""
	// ENV JWT_MAILBOX_KEY=""
	// ENV LOG_FILE=""
	// ENV MYSQL_DATABASE=""
	// ENV MYSQL_HOST=""
	// ENV MYSQL_PASSWORD=""
	// ENV MYSQL_PORT=""
	// ENV MYSQL_USER=""
	// ENV OTEL_EXPORTER_OTLP_ENDPOINT=""
	// ENV OTEL_EXPORTER_OTLP_PROTOCOL=""
	// ENV OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=""
	// ENV OTEL_SERVICE_NAME=""
	// ENV PROMETHEUS_LISTEN_ADDR=""
	// ENV REDIS_ADDR=""
	// ENV REDIS_PWD=""
	// ENV SERVICE_DOC=""
	// ENV SERVICE_ID=""
	// ENV SERVICE_VERSION=""
	os.Setenv("BIND_ADDR", "0.0.0.0:8080")
	os.Setenv("MYSQL_HOST", "127.0.0.1")
	os.Setenv("MYSQL_PORT", "3306")
	os.Setenv("MYSQL_USER", "root")
	os.Setenv("MYSQL_PASSWORD", "helloworld")
	os.Setenv("MYSQL_DATABASE", "test")
	os.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	os.Setenv("REDIS_PWD", "")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
	os.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:4318/v1/traces")
	os.Setenv("JAEGER_AGENT_HOST", "127.0.0.1")
	os.Setenv("JAEGER_AGENT_PORT", "6831")
	os.Setenv("JAEGER_COLLECTOR_ENDPOINT", "http://127.0.0.1:4318")
}
func TestMain(m *testing.M) {
	initEnv()

	_ = log.InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "../tmp/log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "../tmp/err.log"),
		zapcore.InfoLevel,
	)

	err := redis.InitRedis(
		context.Background(),
		redis.LoadRedisConfigFromEnv(),
		1*time.Second,
		2,
	)
	if err != nil {
		panic(err)
	}
	exitCode := m.Run()
	os.Exit(exitCode)
}

// Test case: concurrent Lua executions with occasional key deletes and script flush.
// Expectation: final counter equals total successful increments (no lost updates).
func TestLua_auto_reload(t *testing.T) {
	key := "lua_auto_reload_test_key"
	script := `
return redis.call("INCR", KEYS[1])
	`
	executor := redis.NewLuaExecutor(
		"test",
		redis.GlobalRedis(),
		script,
		nil,
	)
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	N := 1000
	deleteLuaScript(t, ctx)

	for i := 1; i <= N; i++ {
		if rand.Int()%10 < 1 {
			deleteKey(t, ctx, key)
		}
		wg.Add(1)
		go func(expected int) {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			if err := executor.UpdateOneEvent(ctx, []string{key}); err != nil {
				t.Errorf("UpdateOneEvent error: %v", err)
				return
			}
		}(i)
	}

	wg.Wait()
	t.Log("goroutines down")
	val, err := redis.GlobalRedis().Get(ctx, key).Int()
	if err != nil {
		t.Errorf("Get error: %v", err)
		return
	}
	assert.Equal(t, N, val, "Final value should be %d", N)
}

// Test case: script is preloaded, flushed (NOSCRIPT), then executed twice.
// Expectation: executor reloads script and both executions succeed (counter == 2).
func TestLua_reload_after_noscript(t *testing.T) {
	key := "lua_reload_after_noscript_test_key"
	script := `
return redis.call("INCR", KEYS[1])
	`
	executor := redis.NewLuaExecutor(
		"test_reload_after_noscript",
		redis.GlobalRedis(),
		script,
		nil,
	)
	ctx := context.Background()

	deleteKey(t, ctx, key)
	assert.NoError(t, executor.PrepareLuaScript(ctx), "PrepareLuaScript should succeed")
	deleteLuaScript(t, ctx)

	t.Run("reload after NOSCRIPT", func(t *testing.T) {
		assert.NoError(t, executor.UpdateOneEvent(ctx, []string{key}), "first UpdateOneEvent should succeed")
		assert.Eventually(
			t,
			func() bool { return luaScriptReloaded(ctx, executor) },
			5*time.Second,
			100*time.Millisecond,
			"lua script should be reloaded",
		)
		assert.NoError(t, executor.UpdateOneEvent(ctx, []string{key}), "second UpdateOneEvent should succeed")
		val, err := redis.GlobalRedis().Get(ctx, key).Int()
		assert.NoError(t, err, "Get should succeed")
		assert.Equal(t, 2, val, "Final value should be %d", 2)
	})
}

func deleteKey(t *testing.T, ctx context.Context, key string) {
	t.Helper()
	assert.NoError(t, redis.GlobalRedis().Del(ctx, key).Err(), "Failed to delete key %s", key)
}

func deleteLuaScript(t *testing.T, ctx context.Context) {
	t.Helper()
	assert.NoError(t, redis.GlobalRedis().ScriptFlush(ctx).Err(), "ScriptFlush should succeed")
}

func luaScriptReloaded(ctx context.Context, executor *redis.LuaExecutor) bool {
	sha := executor.LuaScriptSha
	if sha == "" {
		return false
	}
	exists, err := redis.GlobalRedis().ScriptExists(ctx, sha).Result()
	if err != nil {
		return false
	}
	return len(exists) > 0 && exists[0]
}
