package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	redis_rate "github.com/go-redis/redis_rate/v10"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/redis"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Test case: limit of 1 request with long period, two back-to-back calls.
// Expectation: first call allowed, second call rate-limited.
func TestRateLimiter_AllowsThenLimits(t *testing.T) {
	ctx := context.Background()
	limit := redis_rate.Limit{Rate: 1, Burst: 1, Period: time.Hour}
	server, client := newRateLimiterTestServer(t, limit)
	defer server.Close()

	t.Run("first request allowed", func(t *testing.T) {
		_, err := client.CallUnary(ctx, connect.NewRequest(&emptypb.Empty{}))
		if err != nil {
			t.Fatalf("expected first request to be allowed, got error: %v", err)
		}
	})

	t.Run("second request limited", func(t *testing.T) {
		_, err := client.CallUnary(ctx, connect.NewRequest(&emptypb.Empty{}))
		if err == nil {
			t.Fatalf("expected second request to be rate-limited, got nil")
		}
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Fatalf("expected CodeResourceExhausted, got %v", connect.CodeOf(err))
		}
	})
}

func newRateLimiterTestServer(
	t *testing.T,
	limit redis_rate.Limit,
) (*httptest.Server, *connect.Client[emptypb.Empty, emptypb.Empty]) {
	t.Helper()
	procedure := fmt.Sprintf(
		"/tests.rate_limiter.v1.TestService%d/Ping",
		time.Now().UnixNano(),
	)
	limiter := interceptor.NewQPSRateLimiter(
		redis.RedisClient{UniversalClient: redis.GlobalRedis()},
		limit,
	)
	mux := http.NewServeMux()
	mux.Handle(
		procedure,
		connect.NewUnaryHandlerSimple(
			procedure,
			func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
				return &emptypb.Empty{}, nil
			},
			connect.WithInterceptors(limiter),
		),
	)
	server := httptest.NewServer(mux)
	client := connect.NewClient[emptypb.Empty, emptypb.Empty](
		server.Client(),
		server.URL+procedure,
	)
	return server, client
}
