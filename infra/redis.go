package infra

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
)

type RedisConfig struct {
	HostPort string
	Pwd      string

	// 0-15, default 0
	DB uint
}

func LoadRedisConfigFromEnv() RedisConfig {
	return RedisConfig{
		HostPort: utils.StrOrDefault(os.Getenv("REDIS_ADDR"), "127.0.0.1:6379"),
		Pwd:      utils.StrOrDefault(os.Getenv("REDIS_PWD"), ""),
		DB:       0,
	}
}

var (
	_global_redis *redis.Client = nil
	_init_once                  = sync.Once{}
)

func GlobalRedis() *redis.Client {
	return _global_redis
}

func InitRedis(ctx context.Context, conf RedisConfig) error {
	var err error
	_init_once.Do(func() {

		GlobalLog().Info(
			"init-redis",
			zap.Any("redis-config", conf),
		)

		_global_redis = redis.NewClient(&redis.Options{
			Addr:     conf.HostPort,
			Password: conf.Pwd,
			DB:       int(conf.DB),
		})

		err = utils.Retry(
			3, 5*time.Second,
			func() error {
				e := _global_redis.Ping().Err()
				if e != nil {
					GlobalLog().Warn("ping redis", zap.Error(e))
				}
				return e
			},
			"init-redis",
		)
	})

	return err
}
