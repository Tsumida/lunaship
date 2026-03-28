package integration

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/log"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

type observabilityHarness struct {
	logPath string
	errPath string

	shutdownOnce sync.Once
}

func initializeObservability(t *testing.T, logPrefix string) *observabilityHarness {
	t.Helper()

	logDir := t.TempDir()
	harness := &observabilityHarness{
		logPath: filepath.Join(logDir, logPrefix+".log"),
		errPath: filepath.Join(logDir, logPrefix+".err.log"),
	}

	_ = log.InitLog(harness.logPath, harness.errPath, zapcore.InfoLevel)
	t.Cleanup(func() {
		harness.Shutdown(t)
	})

	assert.NoError(t, infra.InitTracingFromEnv(), "trace initialization should succeed")
	infra.InitMetricAsync(":0")
	return harness
}

func (h *observabilityHarness) Shutdown(t *testing.T) {
	t.Helper()

	h.shutdownOnce.Do(func() {
		assert.NoError(t, infra.CloseTracing(), "trace shutdown should flush spans cleanly")
		err := log.GlobalLog().Sync()
		if err != nil && !isIgnorableLoggerSyncError(err) {
			assert.NoError(t, err, "logger sync should succeed")
		}
	})
}

func ensureTopicExists(t *testing.T, brokers []string, topic string, cfg *sarama.Config) {
	t.Helper()

	admin, err := sarama.NewClusterAdmin(brokers, cfg)
	assert.NoError(t, err, "kafka admin initialization should succeed")
	if err != nil {
		return
	}
	t.Cleanup(func() {
		_ = admin.Close()
	})

	err = admin.CreateTopic(topic, &sarama.TopicDetail{NumPartitions: 1, ReplicationFactor: 1}, false)
	if err != nil {
		assert.True(t, isTopicAlreadyExistsError(err), "topic create should only fail when topic already exists: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func integrationKafkaBrokerAddr() string {
	return envOrDefault("KAFKA_ADDR", "127.0.0.1:9092")
}

func configureIntegrationTraceExporter(t *testing.T) *otlpTraceCollector {
	t.Helper()

	traceExporterEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if traceExporterEndpoint != "" {
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", traceExporterEndpoint)
		return nil
	}

	collector := newOTLPTraceCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.Endpoint())
	return collector
}

func kafkaTracerName() string {
	name := strings.TrimSpace(os.Getenv("APP_NAME"))
	if name == "" {
		return "lunaship.kafka"
	}
	return name
}

func isTopicAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

type saramaProducerHeaderCarrier struct {
	headers *[]sarama.RecordHeader
}

func (c saramaProducerHeaderCarrier) Get(key string) string {
	for _, header := range *c.headers {
		if strings.EqualFold(string(header.Key), key) {
			return string(header.Value)
		}
	}
	return ""
}

func (c saramaProducerHeaderCarrier) Set(key, value string) {
	for _, header := range *c.headers {
		if strings.EqualFold(string(header.Key), key) {
			header.Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c saramaProducerHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, header := range *c.headers {
		keys = append(keys, string(header.Key))
	}
	return keys
}

type otlpTraceCollector struct {
	server *httptest.Server

	mu    sync.Mutex
	spans []*tracepb.Span
}

func newOTLPTraceCollector(t *testing.T) *otlpTraceCollector {
	t.Helper()

	collector := &otlpTraceCollector{}
	server := httptest.NewServer(http.HandlerFunc(collector.handle))
	collector.server = server
	t.Cleanup(server.Close)
	return collector
}

func (c *otlpTraceCollector) Endpoint() string {
	return c.server.URL
}

func (c *otlpTraceCollector) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/traces" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rawBody, err := readTraceRequestBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	request := &collectortracepb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(rawBody, request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	response, err := proto.Marshal(&collectortracepb.ExportTraceServiceResponse{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.mu.Lock()
	for _, resourceSpans := range request.ResourceSpans {
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			c.spans = append(c.spans, scopeSpans.Spans...)
		}
	}
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

func (c *otlpTraceCollector) waitForSpans(t *testing.T, minimum int, timeout time.Duration) []*tracepb.Span {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		spans := c.snapshot()
		if len(spans) >= minimum {
			return spans
		}
		if time.Now().After(deadline) {
			assert.Failf(t, "trace export timeout", "expected at least %d spans, got %d", minimum, len(spans))
			return spans
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (c *otlpTraceCollector) snapshot() []*tracepb.Span {
	c.mu.Lock()
	defer c.mu.Unlock()

	spans := make([]*tracepb.Span, len(c.spans))
	copy(spans, c.spans)
	return spans
}

func readTraceRequestBody(r *http.Request) ([]byte, error) {
	if r.Header.Get("Content-Encoding") != "gzip" {
		return io.ReadAll(r.Body)
	}

	gzipReader, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gzipReader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func isIgnorableLoggerSyncError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrInvalid) || strings.Contains(strings.ToLower(err.Error()), "invalid argument")
}
