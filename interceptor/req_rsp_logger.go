package interceptor

import (
	"context"
	"sync"

	"connectrpc.com/connect"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

var logFieldsPool = sync.Pool{
	New: func() any {
		fields := make([]zap.Field, 0, 24)
		return &fields
	},
}

// NewLoggerInterceptor attaches base log fields into the context and logs request/response.
// Server-side only: it mutates the context and is not safe for client interceptors.
func NewLoggerInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			var (
				resp connect.AnyResponse
				err  error

				start = utils.NowInMs()
			)
			baseFields := make([]zap.Field, 0, 24)
			baseFields = append(baseFields,
				zap.String("peer", req.Peer().Addr),
				zap.String("protocol", req.Peer().Protocol),
				zap.String("target", req.Spec().Procedure),
				zap.Uint64("start_ms", start),
			)
			ctx = log.WithFields(ctx, baseFields...)

			logger := log.Logger(ctx)
			if log.IsSampled(ctx) {
				reqFields := borrowFields()
				reqFields = append(reqFields, zap.Any("parameters", req.Any()))
				logger.Info("request", reqFields...)
				releaseFields(reqFields)
			}

			defer func() {
				durInMs := utils.NowInMs() - start
				respFields := borrowFields()
				respFields = append(respFields,
					zap.Any("resp", resp),
					zap.Uint64("duration_ms", durInMs),
				)
				if err != nil {
					respFields = append(respFields,
						zap.String("err_code", connect.CodeOf(err).String()),
						zap.Error(err),
					)
					logger.Error("response", respFields...)
					releaseFields(respFields)
					return
				}
				if log.IsSampled(ctx) {
					logger.Info(
						"responese",
						respFields...,
					)
				}
				releaseFields(respFields)
			}()

			resp, err = next(ctx, req)
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func borrowFields() []zap.Field {
	fields := logFieldsPool.Get().(*[]zap.Field)
	return (*fields)[:0]
}

func releaseFields(fields []zap.Field) {
	cleared := fields[:0]
	logFieldsPool.Put(&cleared)
}
