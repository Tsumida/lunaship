package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/tsumida/lunaship/utils"
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
