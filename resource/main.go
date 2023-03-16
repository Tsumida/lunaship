package main

import (
	"fmt"
	"net/http"

	"github.com/bufbuild/connect-go"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/resource/res"
	"github.com/tsumida/lunaship/server"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {

	const address = ":8080"
	path, handler := svc.NewMetaServiceHandler(
		&res.ResourceService{},
		connect.WithRecover(server.RecoverFn),
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
