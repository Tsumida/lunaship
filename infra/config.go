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

func (s *Service) Run(ctx context.Context) {
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

	go PrintRpcCounter(ctx, 1*time.Minute, 10)

	defer GlobalLog().Info("server done")

	mux := http.NewServeMux()
	mux.Handle(s.Path, s.Handler)
	GlobalLog().Info(
		"server up", zap.String("listen", s.BindingAddress),
	)
	http.ListenAndServe(
		s.BindingAddress,
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
