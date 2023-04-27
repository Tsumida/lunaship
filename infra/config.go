package infra

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	ENV_LIVE = "live"
	ENV_TEST = "test"
	ENV_DEV  = "dev"
)

type ServerConfig struct {
	ServiceId         string
	ServiceVersion    string
	ServiceOpenapiDoc string
	DeployEnv         string
}

func (sc *ServerConfig) LoadFromEnv() error {
	sc.ServiceId = os.Getenv("SERVICE_ID")
	sc.ServiceVersion = os.Getenv("SERVICE_VERSION")
	sc.ServiceOpenapiDoc = os.Getenv("SERVICE_DOC")

	return nil
}

type Service struct {
	Handler        http.Handler
	Path           string
	BindingAddress string
}

func (s *Service) Run(
	ctx context.Context,
	shutdownDur time.Duration,
) {
	_ = InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "./tmp/log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "./tmp/err.log"),
		zapcore.InfoLevel,
	)
	defer GlobalLog().Sync()

	utils.PanicIf(
		InitRedis(ctx, LoadRedisConfigFromEnv()),
	)
	defer GlobalRedis().Close()

	defer GlobalLog().Info("server done")

	mux := http.NewServeMux()
	mux.Handle(s.Path, s.Handler)
	GlobalLog().Info(
		"server up", zap.String("listen", s.BindingAddress),
	)

	server := &http.Server{
		Addr:    s.BindingAddress,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go utils.Go(func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				GlobalLog().Error("server down", zap.Error(err))
			} else {
				GlobalLog().Info("server graceful shutdown")
			}
		}
	})

	<-ctx.Done()
	GlobalLog().Info(
		"shutting down server",
		zap.Float64("waiting_sec", shutdownDur.Seconds()),
	)

	sctx, cancel := context.WithTimeout(context.Background(), shutdownDur)
	defer cancel()

	if err := server.Shutdown(sctx); err != nil {
		GlobalLog().Error("failed to shutdown", zap.Error(err))
	}
}

func (s *Service) RunAfterInit(
	ctx context.Context,
	shutdownDur time.Duration,
	initFnList ...func() error,
) {
	_ = InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "./tmp/log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "./tmp/err.log"),
		zapcore.InfoLevel,
	)
	defer GlobalLog().Sync()
	defer GlobalLog().Info("server done")

	for _, initFn := range initFnList {
		if err := initFn(); err != nil {
			panic(err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle(s.Path, s.Handler)
	GlobalLog().Info(
		"server up", zap.String("listen", s.BindingAddress),
	)

	server := &http.Server{
		Addr:    s.BindingAddress,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go utils.Go(func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				GlobalLog().Error("server down", zap.Error(err))
			} else {
				GlobalLog().Info("server graceful shutdown")
			}
		}
	})

	<-ctx.Done()
	GlobalLog().Info(
		"shutting down server",
		zap.Float64("waiting_sec", shutdownDur.Seconds()),
	)

	sctx, cancel := context.WithTimeout(context.Background(), shutdownDur)
	defer cancel()

	if err := server.Shutdown(sctx); err != nil {
		GlobalLog().Error("failed to shutdown", zap.Error(err))
	}
}
