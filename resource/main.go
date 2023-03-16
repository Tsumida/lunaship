package main

import (
	"net/http"
	"os"

	"github.com/bufbuild/connect-go"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/resource/res"
	"github.com/tsumida/lunaship/server"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const address = ":8080"

func main() {
	_ = server.InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "./log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "./err.log"),
		zapcore.InfoLevel,
	)
	defer server.GlobalLog().Sync()

	path, handler := svc.NewResourceServiceHandler(
		&res.ResourceService{},
		connect.WithRecover(server.RecoverFn),
		connect.WithInterceptors(),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server.GlobalLog().Info(
		"server up", zap.String("listen", address),
	)
	http.ListenAndServe(
		address,
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
