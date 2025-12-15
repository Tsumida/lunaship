package redis

import (
	"context"
	"fmt"
	"sync"

	redis "github.com/redis/go-redis/v9"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
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
type LuaExecutor struct {
	name         string
	client       redis.UniversalClient
	LuaScript    string
	LuaScriptSha string
	respFn       func(res any) error

	mx     sync.RWMutex // keep thread-safty
	logger *zap.Logger
}

func NewLuaExecutorWithLogger(
	name string,
	client redis.UniversalClient,
	luaScript string,
	respFn func(res any) error,
	logger *zap.Logger,
) *LuaExecutor {
	return &LuaExecutor{
		name:      name,
		client:    client,
		LuaScript: luaScript,
		respFn:    respFn,
		logger:    logger,
	}
}

func NewLuaExecutor(
	name string,
	client redis.UniversalClient,
	luaScript string,
	respFn func(res any) error,
) *LuaExecutor {
	return NewLuaExecutorWithLogger(name, client, luaScript, respFn, log.GlobalLog().With(zap.String("lua_exec_name", name)))
}

func (l *LuaExecutor) PrepareLuaScript(ctx context.Context) error {
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

func (l *LuaExecutor) fastPath(
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
	}
	return err
}

func (l *LuaExecutor) slowPathWithLock(
	ctx context.Context,
	keys []string,
	scriptArgs ...any,
) error {
	// 走兜底逻辑, 网络流量会更大
	res, err := l.client.Eval(ctx, l.LuaScript, keys, scriptArgs...).Result()
	if err != nil {
		return err
	}
	if l.respFn != nil {
		return l.respFn(res)
	}
	if resInt, _ := res.(int64); resInt == 1 {
		log.GlobalLog().Debug("updated redis", zap.Strings("key", keys))
	}

	// 异步修复
	go utils.GoWithAction(func() {
		l.mx.Lock()
		defer l.mx.Unlock()
		if l.LuaScriptSha != "" {
			return
		}

		log.GlobalLog().Warn("redis script not found, reloading", zap.String("sha", l.LuaScriptSha))
		if err := l.PrepareLuaScript(ctx); err != nil {
			log.GlobalLog().Error("failed to reload lua script", zap.Error(err))
		}
	}, func(r any) {
		log.GlobalLog().Error("lua script reload panic", zap.String("name", l.name), zap.Strings("keys", keys), zap.Any("args", scriptArgs), zap.Any("recover", r))
	})

	return err
}

func (l *LuaExecutor) UpdateOneEvent(
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

	return l.slowPathWithLock(ctx, keys, scriptArgs...)
}
