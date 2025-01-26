package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/go-redis/redis_rate"
	"github.com/tsumida/lunaship/infra"
)

func NewRateLimiter(
	QpsLimit uint64,
	Dur time.Duration,
) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			url := req.Spec().Procedure
			peer := strings.Split(req.Peer().Addr, ":")[0]
			field := fmt.Sprintf("%s#%s", peer, url)

			_, _, allowed := redis_rate.
				NewLimiter(infra.GlobalRedis()).
				Allow(field, int64(QpsLimit), Dur)
			if !allowed {
				return nil, connect.NewError(
					connect.CodeResourceExhausted,
					errors.New("rate-limited"),
				)
			}

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
