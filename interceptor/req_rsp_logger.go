package interceptor

import (
	"context"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

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
				fields = []zap.Field{
					zap.String("peer", req.Peer().Addr),
					zap.String("protocol", req.Peer().Protocol),
					zap.String("target", req.Spec().Procedure),
					zap.Any("parameters", req.Any()),
					zap.Uint64("start_ms", start),
				}
			)
			if traceID, spanID, ok := log.TraceFromContext(ctx); ok {
				fields = append(fields,
					zap.String("trace_id", traceID),
					zap.String("span_id", spanID),
				)
			}
			logger := log.Logger(ctx)
			if log.IsSampled(ctx) {
				logger.Info("request", fields...)
			}

			defer func() {
				durInMs := utils.NowInMs() - start
				fields = append(fields,
					zap.Any("resp", resp),
					zap.Uint64("duration_ms", durInMs),
				)
				if err != nil {
					fields = append(fields,
						zap.String("err_code", connect.CodeOf(err).String()),
						zap.Error(err),
					)
					logger.Error("response", fields...)
					return
				}
				if log.IsSampled(ctx) {
					logger.Info(
						"responese",
						fields...,
					)
				}
			}()

			resp, err = next(ctx, req)
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
