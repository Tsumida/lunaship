package interceptor

import (
	"context"
	"sort"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/samber/lo"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/redis"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

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
				client := redis.GlobalRedis()
				res := client.HIncrBy(ctx, KEY_RPC_COUNTER, url, 1)
				if err := res.Err(); err != nil {
					log.GlobalLog().Error(
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

var (
	// key=string, valuee=uint64
	RPC_COUNTER = sync.Map{}
)

func PrintRpcCounter(ctx context.Context, interval time.Duration, topk uint) {
	run := true
	for run {
		select {
		case <-ctx.Done():
			log.GlobalLog().Info("rpc_counter done")
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

			log.GlobalLog().Info(
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
