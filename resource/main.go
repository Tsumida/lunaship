package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bufbuild/connect-go"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/utils"

	"github.com/tsumida/lunaship/resource/res"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func RecoverFn(
	ctx context.Context,
	spec connect.Spec,
	header http.Header,
	recoverValue any,
) error {

	now := utils.NowInSec()
	fmt.Printf(
		"[%d][%s]panic=%v\n", now, spec.Procedure, recoverValue,
	)
	return connect.NewError(
		connect.CodeInternal, nil,
	)
}

func main() {

	const address = ":8080"
	path, handler := svc.NewMetaServiceHandler(
		&res.ResourceService{},
		connect.WithRecover(RecoverFn),
		connect.WithInterceptors(),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	fmt.Println("... Listening on", address)
	http.ListenAndServe(
		address,
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
