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
				zap.String("_remote_service", remoteServiceFromHeader(req.Header().Get(remoteServiceHeader))),
				zap.String("_remote_endpoint", req.Spec().Procedure),
				zap.String("_remote_ip_port", req.Peer().Addr),
				zap.String("_rpc_protocol", req.Peer().Protocol),
				zap.String("_http_method", req.HTTPMethod()),
				zap.Uint64("_start_ms", start),
			)
			logger := log.Logger(ctx).With(baseFields...)
			if log.IsSampled(ctx) {
				reqFields := borrowFields()
				reqFields = append(reqFields, zap.Any("_parameters", req.Any()))
				logger.Info("request", reqFields...)
				releaseFields(reqFields)
			}

			defer func() {
				durInMs := utils.NowInMs() - start
				respFields := borrowFields()
				respFields = append(respFields,
					zap.Any("_resp", resp),
					zap.Uint64("_dur_ms", durInMs),
				)
				if err != nil {
					respFields = append(respFields,
						zap.String("_err_code", connect.CodeOf(err).String()),
						zap.NamedError("_error", err),
					)
					logger.Error("response", respFields...)
					releaseFields(respFields)
					return
				}
				if log.IsSampled(ctx) {
					logger.Info(
						"response",
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
