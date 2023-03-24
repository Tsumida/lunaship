package infra

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/go-redis/redis_rate"
	"github.com/samber/lo"
	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
)

func NewReqRespLogger() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {

			var (
				resp   connect.AnyResponse
				err    error
				fields = []zap.Field{
					zap.String("peer", req.Peer().Addr),
					zap.String("protocol", req.Peer().Protocol),
					zap.String("target", req.Spec().Procedure),
					zap.Any("parameters", req.Any()),
				}
			)
			GlobalLog().Info(
				"request",
				fields...,
			)
			start := utils.NowInMs()

			defer func() {
				durInMs := utils.NowInMs() - start
				fields = append(fields,
					zap.Any("resp", resp),
					zap.Uint64("duration_ms", durInMs),
				)
				if err != nil {
					fields = append(fields, zap.Error(err))
				}
				GlobalLog().Info(
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

var (
	// key=string, valuee=uint64
	RPC_COUNTER = sync.Map{}
)

func PrintRpcCounter(ctx context.Context, interval time.Duration, topk uint) {
	run := true
	for run {
		select {
		case <-ctx.Done():
			GlobalLog().Info("rpc_counter done")
			run = false
		default:
			time.Sleep(interval)
			mapper := make(map[string]uint64, 64)
			RPC_COUNTER.Range(func(key, value any) bool {
				mapper[key.(string)] = value.(uint64)
				return true
			})

			entries := lo.Entries(mapper)
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Value < entries[j].Value
			})
			tail := topk
			if tail > uint(len(entries)) {
				tail = uint(len(entries))
			}

			GlobalLog().Info(
				"rpc_counter", zap.Any(
					"counters", entries[:tail],
				),
			)
		}
	}
}

func NewLocalRpcCounter() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			key := req.Spec().Procedure
			val := uint64(1)
			if cnt, ok := RPC_COUNTER.Load(key); ok {
				val += cnt.(uint64)
			}

			RPC_COUNTER.Store(key, val)

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

var (
	KEY_RPC_COUNTER = "rpc_counter"
)

func NewRedisRpcCounter() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			url := req.Spec().Procedure
			go utils.Go(func() {
				client := GlobalRedis()
				res := client.HIncrBy(KEY_RPC_COUNTER, url, 1)
				if err := res.Err(); err != nil {
					GlobalLog().Error(
						KEY_RPC_COUNTER,
						zap.String("url", url),
						zap.Error(err),
					)
				}
			})

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

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
				NewLimiter(GlobalRedis()).
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
