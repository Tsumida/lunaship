package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/example/logs/gen/logsv1connect"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/service"
	"github.com/tsumida/lunaship/utils"
)

func main() {
	ensureAppConfigPath()

	bindAddr := utils.StrOrDefault(os.Getenv("BIND_ADDR"), ":8080")

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

	s.Run(context.Background())
}

func ensureAppConfigPath() {
	if strings.TrimSpace(os.Getenv("APP_CONFIG_PATH")) != "" {
		return
	}

	candidates := []string{
		filepath.Join("config", "app.toml"),
		filepath.Join("example", "logs", "config", "app.toml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			_ = os.Setenv("APP_CONFIG_PATH", candidate)
			return
		}
	}
}
