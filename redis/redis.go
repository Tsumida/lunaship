package redis

import (
	"context"
	"os"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

type RedisClient struct {
	redis.UniversalClient
}

type RedisConfig struct {
	HostPort string
	Pwd      string

	// 0-15, default 0
	DB uint
}

func LoadRedisConfigFromEnv() *redis.UniversalOptions {
	return &redis.UniversalOptions{
		Addrs: []string{
			utils.StrOrDefault(os.Getenv("REDIS_ADDR"), "127.0.0.1:6379"),
		},
		Password: utils.StrOrDefault(os.Getenv("REDIS_PWD"), ""),
		DB:       0,
	}
}

var (
	globalRedis     redis.UniversalClient = nil
	initGlobalRedis                       = sync.Once{}
)

func GlobalRedis() redis.UniversalClient {
	return globalRedis
}

func InitRedis(ctx context.Context, conf *redis.UniversalOptions, timeout time.Duration, retry uint) error {
	var err error
	initGlobalRedis.Do(func() {
		globalRedis = redis.NewUniversalClient(conf)
		err = utils.Retry(retry, timeout,
			func() error {
				e := globalRedis.Ping(ctx).Err()
				if e != nil {
					log.GlobalLog().Warn("ping redis", zap.Error(e))
				}
				return e
			},
			"init-redis",
		)
	})

	return err
}
