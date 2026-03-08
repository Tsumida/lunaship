package interceptor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/types/known/emptypb"
)

var traceIDRegexp = regexp.MustCompile(`_trace_id[^0-9a-fA-F]*([0-9a-fA-F]+)`)

// Description:
// Start two services (A -> B). Service A receives a request and calls service B.
//
// Expectation:
// Log entries from service A and service B share the same _trace_id.
func TestTracePropagationRPC(t *testing.T) {
	logPath := initTestLogger(t)
	initTestTracer(t)

	procedureA := "/tests.trace.v1.ServiceA/Ping"
	procedureB := "/tests.trace.v1.ServiceB/Ping"

	serverB := newTraceServer(t, procedureB, func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
		return &emptypb.Empty{}, nil
	})
	defer serverB.Close()

	clientToB := newTraceClient(t, serverB, procedureB, true)
	serverA := newTraceServer(t, procedureA, func(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
		_, err := clientToB.CallUnary(ctx, connect.NewRequest(&emptypb.Empty{}))
		return &emptypb.Empty{}, err
	})
	defer serverA.Close()

	clientToA := newTraceClient(t, serverA, procedureA, false)

	t.Run("flow: service_a calls service_b", func(t *testing.T) {
		// Description: service A receives a request and calls service B.
		// Expectation: both services log the same _trace_id.
		_, err := clientToA.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{}))
		assert.NoError(t, err)
		_ = log.GlobalLog().Sync()

		traceA, lineA, err := traceIDFromLogFile(logPath, procedureA)
		assert.NoError(t, err)
		traceB, lineB, err := traceIDFromLogFile(logPath, procedureB)
		assert.NoError(t, err)
		assert.NotEmpty(t, traceA)
		assert.Equal(t, traceA, traceB)
		t.Logf("service_a log: %s", lineA)
		t.Logf("service_b log: %s", lineB)
		t.Logf("service_a _trace_id=%s service_b _trace_id=%s", traceA, traceB)
	})
}

func initTestLogger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "log.log")
	errPath := filepath.Join(dir, "err.log")
	log.InitLog(logPath, errPath, zapcore.InfoLevel)
	return logPath
}

func initTestTracer(t *testing.T) {
	t.Helper()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
}

func newTraceServer(
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
			connect.WithInterceptors(
				NewTraceInterceptor(),
				NewLoggerInterceptor(),
			),
		),
	)
	return httptest.NewServer(mux)
}

func newTraceClient(
	t *testing.T,
	server *httptest.Server,
	procedure string,
	withTracing bool,
) *connect.Client[emptypb.Empty, emptypb.Empty] {
	t.Helper()
	opts := []connect.ClientOption{}
	if withTracing {
		opts = append(opts, connect.WithInterceptors(NewTraceClientInterceptor()))
	}
	return connect.NewClient[emptypb.Empty, emptypb.Empty](
		server.Client(),
		server.URL+procedure,
		opts...,
	)
}

func traceIDFromLogFile(logPath, target string) (string, string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, target) {
			continue
		}
		match := traceIDRegexp.FindStringSubmatch(line)
		if len(match) > 1 {
			return match[1], line, nil
		}
	}
	return "", "", fmt.Errorf("_trace_id not found for target %s", target)
}
