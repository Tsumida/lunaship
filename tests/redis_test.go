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

func TestMain(m *testing.M) {
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

func TestLua_auto_reload(t *testing.T) {
	// 目标: 如果脚本被删除、或者未初始化，也能正常执行。
	// 测试: 初始值为0，每次递增1， 执行N次后，值应为N。

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

func deleteKey(t *testing.T, ctx context.Context, key string) {
	t.Helper()
	if err := redis.GlobalRedis().Del(ctx, key).Err(); err != nil {
		t.Fatalf("Failed to delete key %s: %v", key, err)
	}
}

func deleteLuaScript(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := redis.GlobalRedis().ScriptFlush(ctx).Err(); err != nil {
		panic(err)
	}
}
