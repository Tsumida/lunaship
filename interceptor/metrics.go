package interceptor

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tsumida/lunaship/utils"
)

var (
	rpcServerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_server_requests_total",
			Help: "Total number of unary RPC server requests by endpoint and code.",
		},
		[]string{"service", "endpoint", "code"},
	)
	rpcServerRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_server_request_duration_seconds",
			Help:    "Unary RPC server request duration in seconds by endpoint and code.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "endpoint", "code"},
	)
)

// Static register
func RegisterMetric() {
	registerCollector(rpcServerRequestsTotal)
	registerCollector(rpcServerRequestDurationSeconds)
}

// May panic
func NewMetricsInterceptor() connect.UnaryInterceptorFunc {
	RegisterMetric()
	serviceName := serviceNameFromEnv()
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			code := connect.CodeOf(err).String()
			endpoint := req.Spec().Procedure

			rpcServerRequestsTotal.WithLabelValues(serviceName, endpoint, code).Inc()
			rpcServerRequestDurationSeconds.WithLabelValues(serviceName, endpoint, code).Observe(time.Since(start).Seconds())
			return resp, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func serviceNameFromEnv() string {
	service := strings.TrimSpace(os.Getenv("SERVICE_ID"))
	return utils.StrOrDefault(service, "lunaship")
}

func registerCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		var alreadyRegisteredErr prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegisteredErr) {
			return
		}
		panic(err)
	}
}
