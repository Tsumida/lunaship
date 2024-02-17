package infra

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

	GlobalLog().Error(
		"request panic",
		zap.String("target", spec.Procedure),
		zap.Any("recover", recoverValue),
		zap.Stack("stack"),
	)
	return connect.NewError(
		connect.CodeInternal, nil,
	)
}
