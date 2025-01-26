package middleware

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
)

func logFields(req connect.AnyRequest) []zap.Field {
	return append(
		make([]zap.Field, 0, 8),
		[]zap.Field{
			zap.String("peer", req.Peer().Addr),
			zap.String("protocol", req.Peer().Protocol),
			zap.String("target", req.Spec().Procedure),
			zap.Any("parameters", req.Any()),
		}...,
	)
}

func NewReqRespLogger() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {

			var (
				resp connect.AnyResponse
				err  error

				start  = utils.NowInMs()
				fields = append(logFields(req), zap.Uint64("start_ms", start))
			)
			infra.GlobalLog().Info("request", fields...)

			defer func() {
				durInMs := utils.NowInMs() - start
				fields = append(fields,
					zap.Any("resp", resp),
					zap.Uint64("duration_ms", durInMs),
				)
				if err != nil {
					fields = append(fields, zap.Error(err))
				}
				infra.GlobalLog().Info(
					"responese",
					fields...,
				)
			}()

			resp, err = next(ctx, req)
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
