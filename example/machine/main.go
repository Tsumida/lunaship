package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/bufbuild/connect-go"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/example/machine/res"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const address = ":8080"

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer cancel()
	_ = infra.InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "./tmp/log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "./tmp/err.log"),
		zapcore.InfoLevel,
	)
	defer infra.GlobalLog().Sync()

	utils.PanicIf(
		infra.InitRedis(ctx, infra.LoadRedisConfigFromEnv()),
	)
	defer infra.GlobalRedis().Close()

	path, handler := svc.NewResourceServiceHandler(
		&res.ResourceService{},
		connect.WithRecover(infra.RecoverFn),
		connect.WithInterceptors(
			infra.NewLocalRpcCounter(),
			infra.NewReqRespLogger(),
		),
	)

	go infra.PrintRpcCounter(ctx, 1*time.Minute, 10)

	defer infra.GlobalLog().Info("server done")

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	infra.GlobalLog().Info(
		"server up", zap.String("listen", address),
	)
	http.ListenAndServe(
		address,
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
