package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	redis_rate "github.com/go-redis/redis_rate/v10"
	"github.com/tsumida/lunaship/redis"
)

func NewQPSRateLimiter(
	QpsLimit uint64,
	redisClient redis.RedisClient,
	reqPerSec int,
) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			url := req.Spec().Procedure
			peer := strings.Split(req.Peer().Addr, ":")[0]
			key := fmt.Sprintf("%s#%s", peer, url)
			result, err := redis_rate.
				NewLimiter(redisClient).
				Allow(ctx, key, redis_rate.PerSecond(reqPerSec))
			if err != nil {

				return nil, connect.NewError(
					connect.CodeResourceExhausted,
					errors.New("rate-limited"),
				)
			}
			if result.Allowed > 0 {
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
