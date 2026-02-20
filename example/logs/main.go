package main

import (
	"context"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/example/logs/gen/logsv1connect"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/service"
)

func main() {
	_ = os.Setenv("LOG_FILE", "./tmp/log.log")
	_ = os.Setenv("ERR_FILE", "./tmp/err.log")

	path, handler := logsv1connect.NewDummyServiceHandler(
		NewDummyService(),
		connect.WithRecover(infra.RecoverFn),
		connect.WithInterceptors(
			interceptor.NewTraceInterceptor(),
			interceptor.NewLoggerInterceptor(),
		),
	)

	s := service.Service{
		Handler:        handler,
		Path:           path,
		BindingAddress: ":8080",
	}

	s.RunAfterInit(context.Background(), 10*time.Second)
}
