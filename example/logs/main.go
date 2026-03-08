package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/example/logs/gen/logsv1connect"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/redis"
	"github.com/tsumida/lunaship/service"
	"github.com/tsumida/lunaship/utils"
	"gorm.io/gorm"
)

func main() {
	if os.Getenv("LOG_FILE") == "" {
		_ = os.Setenv("LOG_FILE", "./tmp/log.log")
	}
	if os.Getenv("ERR_FILE") == "" {
		_ = os.Setenv("ERR_FILE", "./tmp/err.log")
	}
	bindAddr := utils.StrOrDefault(os.Getenv("BIND_ADDR"), ":8080")
	mysqlConf := infra.LoadMySQLConfFromEnv(false)
	redisConf := redis.LoadRedisConfigFromEnv()

	path, handler := logsv1connect.NewDummyServiceHandler(
		NewDummyService(),
		connect.WithRecover(infra.RecoverFn),
		connect.WithInterceptors(
			interceptor.NewMetricsInterceptor(),
			interceptor.NewTraceInterceptor(),
			interceptor.NewLoggerInterceptor(),
		),
	)

	s := service.Service{
		Handler:        handler,
		Path:           path,
		BindingAddress: bindAddr,
	}

	s.RunAfterInit(context.Background(), 10*time.Second,
		func() error {
			infra.InitMetricAsync(infra.PROMETHEUS_LISTEN_ADDR)
			return nil
		},
		func() error {
			if err := infra.InitMySQL(
				mysqlConf,
				gorm.Config{
					Logger: infra.NewMySQLGormLogger(mysqlConf),
				},
				func(db *gorm.DB) error {
					if db == nil {
						return fmt.Errorf("nil mysql db")
					}
					return nil
				},
			); err != nil {
				return err
			}
			return redis.InitRedis(context.Background(), redisConf, time.Second, 2)
		},
	)
}
