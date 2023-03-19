package infra

import (
	"context"
	"os"
	"sync"

	"github.com/go-redis/redis"
	"github.com/tsumida/lunaship/utils"
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

func InitRedis(ctx context.Context, conf RedisConfig) {
	_init_once.Do(func() {
		_global_redis = redis.NewClient(&redis.Options{
			Addr:     conf.HostPort,
			Password: conf.Pwd,
			DB:       int(conf.DB),
		})
	})
}
