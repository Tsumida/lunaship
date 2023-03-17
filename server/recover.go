package server

import (
	"context"
	"net/http"

	"github.com/bufbuild/connect-go"
	"go.uber.org/zap"
)

func RecoverFn(
	ctx context.Context,
	spec connect.Spec,
	header http.Header,
	recoverValue any,
) error {

	GlobalLog().Warn("request panic", zap.Any("recover", recoverValue))
	return connect.NewError(
		connect.CodeInternal, nil,
	)
}
