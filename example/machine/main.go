package main

import (
	"context"

	"github.com/bufbuild/connect-go"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/example/machine/res"
	"github.com/tsumida/lunaship/infra"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer cancel()
	path, handler := svc.NewResourceServiceHandler(
		&res.ResourceService{},
		connect.WithRecover(infra.RecoverFn),
		connect.WithInterceptors(
			// infra.NewRateLimiter(),
			infra.NewRedisRpcCounter(),
			infra.NewReqRespLogger(),
		),
	)

	svc := &infra.Service{
		Path:           path,
		Handler:        handler,
		BindingAddress: ":8080",
	}

	svc.Run(ctx)
}
