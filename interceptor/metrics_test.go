package interceptor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Description:
// Send a successful unary RPC request through metrics interceptor.
//
// Expectation:
// Request counter increments with code=ok and duration histogram receives one sample.
func TestMetricsInterceptor_Success(t *testing.T) {
	t.Setenv("SERVICE_ID", "metrics-test-success")
	resetRPCMetrics()

	procedure := fmt.Sprintf("/tests.metrics.v1.SuccessService%d/Ping", time.Now().UnixNano())
	server := newMetricsServer(t, procedure, func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
		return &emptypb.Empty{}, nil
	})
	defer server.Close()

	client := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+procedure)
	_, err := client.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	assert.NoError(t, err)

	okCode := connect.CodeOf(nil).String()
	counter, err := rpcServerRequestsTotal.GetMetricWithLabelValues("metrics-test-success", procedure, okCode)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, testutil.ToFloat64(counter))

	histCount := histogramSampleCount(t, rpcServerRequestDurationSeconds, "metrics-test-success", procedure, okCode)
	assert.Equal(t, uint64(1), histCount)
}

// Description:
// Send an error unary RPC request through metrics interceptor.
//
// Expectation:
// Request counter increments with code=invalid_argument and duration histogram receives one sample.
func TestMetricsInterceptor_Error(t *testing.T) {
	t.Setenv("SERVICE_ID", "metrics-test-error")
	resetRPCMetrics()

	procedure := fmt.Sprintf("/tests.metrics.v1.ErrorService%d/Ping", time.Now().UnixNano())
	server := newMetricsServer(t, procedure, func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("bad request"))
	})
	defer server.Close()

	client := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+procedure)
	_, err := client.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	assert.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	counter, err := rpcServerRequestsTotal.GetMetricWithLabelValues("metrics-test-error", procedure, connect.CodeInvalidArgument.String())
	assert.NoError(t, err)
	assert.Equal(t, 1.0, testutil.ToFloat64(counter))

	histCount := histogramSampleCount(t, rpcServerRequestDurationSeconds, "metrics-test-error", procedure, connect.CodeInvalidArgument.String())
	assert.Equal(t, uint64(1), histCount)
}

func newMetricsServer(
	t *testing.T,
	procedure string,
	handler func(context.Context, *emptypb.Empty) (*emptypb.Empty, error),
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(
		procedure,
		connect.NewUnaryHandlerSimple(
			procedure,
			handler,
			connect.WithInterceptors(NewMetricsInterceptor()),
		),
	)
	return httptest.NewServer(mux)
}

func histogramSampleCount(
	t *testing.T,
	histVec *prometheus.HistogramVec,
	service string,
	endpoint string,
	code string,
) uint64 {
	t.Helper()
	observer, err := histVec.GetMetricWithLabelValues(service, endpoint, code)
	assert.NoError(t, err)

	metric, ok := observer.(prometheus.Metric)
	assert.True(t, ok)

	dtoMetric := &dto.Metric{}
	err = metric.Write(dtoMetric)
	assert.NoError(t, err)
	return dtoMetric.GetHistogram().GetSampleCount()
}

func resetRPCMetrics() {
	rpcServerRequestsTotal.Reset()
	rpcServerRequestDurationSeconds.Reset()
}
