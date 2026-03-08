package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/interceptor"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Description:
// Expose prometheus metrics endpoint and send both successful and failed RPC requests.
//
// Expectation:
// /metrics contains request counter and duration histogram families with expected labels.
func TestMetricsEndpoint_ExposesRPCMetrics(t *testing.T) {
	t.Setenv("SERVICE_ID", "metrics-integration")

	successProcedure := fmt.Sprintf("/tests.metrics.v1.IntegrationSuccess%d/Ping", time.Now().UnixNano())
	errorProcedure := fmt.Sprintf("/tests.metrics.v1.IntegrationError%d/Ping", time.Now().UnixNano())
	server := newMetricsIntegrationServer(t, successProcedure, errorProcedure)
	defer server.Close()

	successClient := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+successProcedure)
	errorClient := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+errorProcedure)

	_, err := successClient.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	assert.NoError(t, err)

	_, err = errorClient.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	assert.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	resp, err := server.Client().Get(server.URL + "/metrics")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	text := string(data)

	assert.Contains(t, text, "rpc_server_requests_total")
	assert.Contains(t, text, "rpc_server_request_duration_seconds_bucket")
	assert.Contains(t, text, `service="metrics-integration"`)
	assert.Contains(t, text, `endpoint="`+successProcedure+`"`)
	assert.Contains(t, text, `endpoint="`+errorProcedure+`"`)
	assert.Contains(t, text, `code="`+connect.CodeOf(nil).String()+`"`)
	assert.Contains(t, text, `code="`+connect.CodeInvalidArgument.String()+`"`)
}

func newMetricsIntegrationServer(
	t *testing.T,
	successProcedure string,
	errorProcedure string,
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	interceptorOption := connect.WithInterceptors(interceptor.NewMetricsInterceptor())
	mux.Handle(
		successProcedure,
		connect.NewUnaryHandlerSimple(
			successProcedure,
			func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
				return &emptypb.Empty{}, nil
			},
			interceptorOption,
		),
	)
	mux.Handle(
		errorProcedure,
		connect.NewUnaryHandlerSimple(
			errorProcedure,
			func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid input"))
			},
			interceptorOption,
		),
	)
	mux.Handle("/metrics", promhttp.Handler())
	return httptest.NewServer(mux)
}
