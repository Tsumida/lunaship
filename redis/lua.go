package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"

	redis "github.com/redis/go-redis/v9"
	"github.com/tsumida/lunaship/log"
	"go.uber.org/zap"
)

// Require: Thread-safe
type LuaExecutorAPI interface {
	// APP初始化时调用, 预加载lua脚本并记录sha，后续调用都通过EvalSha执行，降低网络传输开销
	PrepareLuaScript(ctx context.Context) error

	// 通过EvalSha执行lua脚本更新redis状态。如果发现脚本不存在，尝试重新加载脚本
	UpdateOneEvent(
		ctx context.Context,
		keys []string,
		scriptArgs ...any,
	) error
}

// redis对象管理
type LuaExecutor[T any] struct {
	name         string
	client       redis.UniversalClient
	LuaScript    string
	LuaScriptSha string
	respFn       func(res any) error

	mx     sync.RWMutex // keep thread-safty
	logger *zap.Logger
}

func NewLuaExecutorWithLogger[T any](
	name string,
	client redis.UniversalClient,
	luaScript string,
	respFn func(res any) error,
	logger *zap.Logger,
) *LuaExecutor[T] {
	return &LuaExecutor[T]{
		name:      name,
		client:    client,
		LuaScript: luaScript,
		respFn:    respFn,
		logger:    logger,
	}
}

func NewLuaExecutor[T any](
	name string,
	client redis.UniversalClient,
	luaScript string,
	respFn func(res any) error,
) *LuaExecutor[T] {
	return NewLuaExecutorWithLogger[T](name, client, luaScript, respFn, log.GlobalLog().With(zap.String("lua_exec_name", name)))
}

func (l *LuaExecutor[T]) PrepareLuaScript(ctx context.Context) error {
	if l.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	// load script into the configured redis client
	sha, err := l.client.ScriptLoad(ctx, l.LuaScript).Result()
	if err != nil {
		log.GlobalLog().Error("failed to reload lua script", zap.Error(err))
		return err
	}
	l.LuaScriptSha = sha
	log.GlobalLog().Info("Redis script loaded", zap.String("name", l.name), zap.String("sha", l.LuaScriptSha))
	return nil
}

func (l *LuaExecutor[T]) fastPath(
	ctx context.Context,
	keys []string,
	scriptArgs ...any,
) error {
	if l.LuaScriptSha == "" {
		return fmt.Errorf("lua script sha is empty")
	}
	res, err := l.client.EvalSha(ctx, l.LuaScriptSha, keys, scriptArgs...).Result()
	if err == nil {
		if l.respFn != nil {
			return l.respFn(res)
		}
		// 默认处理
		if resInt, _ := res.(int64); resInt == 1 {
			log.GlobalLog().Debug("updated redis", zap.Strings("key", keys))
		} else {
			log.GlobalLog().Info("skipped outdated event", zap.Strings("key", keys), zap.Any("data", scriptArgs))
		}
	}
	return err
}

func (l *LuaExecutor[T]) slowPathWithLock(
	ctx context.Context,
	keys []string,
	scriptArgs ...any,
) error {
	l.mx.Lock()
	defer l.mx.Unlock()
	// double check
	if err := l.fastPath(ctx, keys, scriptArgs...); err == nil {
		return nil
	}
	log.GlobalLog().Warn("redis script not found, reloading", zap.String("sha", l.LuaScriptSha))
	if err := l.PrepareLuaScript(ctx); err != nil {
		log.GlobalLog().Error("failed to reload lua script", zap.Error(err))
		return err
	}
	return l.fastPath(ctx, keys, scriptArgs...)
}

func (l *LuaExecutor[T]) UpdateOneEvent(
	ctx context.Context,
	keys []string,
	scriptArgs ...any,
) error {
	log.GlobalLog().Debug("exec lua script", zap.Strings("keys", keys), zap.Any("args", scriptArgs))
	l.mx.RLock()
	err := l.fastPath(ctx, keys, scriptArgs...)
	l.mx.RUnlock()
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "NOSCRIPT") { // 修正了原代码中对 err 的检查
		return l.slowPathWithLock(ctx, keys, scriptArgs...)
	}
	return err
}
