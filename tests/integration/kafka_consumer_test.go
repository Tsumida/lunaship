package integration

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/kafka"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Description:
// Initialize observability, produce hello world messages to Kafka topic test-lunaship,
// and consume the full topic backlog up to the end offset captured after production.
//
// Expectation:
// The consumer reaches the last known offset and every freshly produced message carries "hello, world".
func TestKafkaConsumer_ConsumesAllMessagesFromTopic(t *testing.T) {
	t.Setenv("APP_NAME", "kafka-consumer")
	t.Setenv("SERVICE_ID", "kafka-consumer")
	t.Setenv("TRACE_ENABLED", "true")
	t.Setenv("TRACE_ERROR_LOG_DISABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	traceExporterEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	var traceCollector *otlpTraceCollector
	if traceExporterEndpoint == "" {
		traceCollector = newOTLPTraceCollector(t)
		traceExporterEndpoint = "http://127.0.0.1:4318"
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", traceExporterEndpoint)

	observability := initializeObservability(t, "consumer")

	brokerAddr := envOrDefault("KAFKA_ADDR", "kafka:9092")
	topic := "test-lunaship"
	payload := "hello, world"
	producedCount := 3

	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 3
	producerConfig.Net.DialTimeout = 3 * time.Second
	producerConfig.Net.ReadTimeout = 3 * time.Second
	producerConfig.Net.WriteTimeout = 3 * time.Second

	ensureTopicExists(t, []string{brokerAddr}, topic, producerConfig)

	client, err := sarama.NewClient([]string{brokerAddr}, producerConfig)
	assert.NoError(t, err, "kafka client initialization should succeed")
	if err != nil {
		return
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	startOffset, err := client.GetOffset(topic, 0, sarama.OffsetNewest)
	assert.NoError(t, err, "start offset lookup should succeed")
	if err != nil {
		return
	}

	producer, err := sarama.NewSyncProducerFromClient(client)
	assert.NoError(t, err, "sync producer initialization should succeed")
	if err != nil {
		return
	}
	t.Cleanup(func() {
		_ = producer.Close()
	})

	for i := 0; i < producedCount; i++ {
		partition, offset, sendErr := produceWithObservability(
			context.Background(),
			producer,
			brokerAddr,
			topic,
			fmt.Sprintf("consumer-msg-%d", i),
			payload,
		)
		assert.NoError(t, sendErr, "message send should succeed")
		assert.Equal(t, int32(0), partition, "integration topic should keep a single partition")
		assert.GreaterOrEqual(t, offset, startOffset, "freshly produced offset should be within the observed range")
	}

	endOffset, err := client.GetOffset(topic, 0, sarama.OffsetNewest)
	assert.NoError(t, err, "end offset lookup should succeed")
	assert.Equal(t, startOffset+int64(producedCount), endOffset, "newest offset should move by the produced message count")
	if err != nil {
		return
	}

	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Net.DialTimeout = 3 * time.Second
	consumerConfig.Net.ReadTimeout = 3 * time.Second
	consumerConfig.Net.WriteTimeout = 3 * time.Second

	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	groupID := fmt.Sprintf("kafka-consumer-integration-%d", time.Now().UnixNano())
	testConsumer := &kafka.KafkaConsumer{
		Brokers:       brokerAddr,
		ConsumerGroup: groupID,
		Topic:         topic,
	}

	state := consumedMessageWindow{
		payloads: make(map[int64]string, producedCount),
	}
	lastExpectedOffset := endOffset - 1
	handlerName := "integration-consumer"
	handler := kafka.NewContextConsumerWrapper(handlerName, func(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
		state.record(startOffset, endOffset, payload, msg)
		if msg.Offset >= lastExpectedOffset {
			cancel()
		}
		return nil
	})

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- testConsumer.Start(consumerCtx, consumerConfig, handler)
	}()

	select {
	case runErr := <-runErrCh:
		assert.NoError(t, runErr, "consumer should shut down cleanly after the context is cancelled")
	case <-time.After(20 * time.Second):
		cancel()
		t.Fatal("consumer should finish reading the topic backlog before timeout")
	}

	unexpectedPayloadErr, consumedOffsets := state.snapshot()
	assert.NoError(t, unexpectedPayloadErr, "freshly produced messages should keep the expected payload")
	assert.Len(t, consumedOffsets, producedCount, "consumer should observe every freshly produced message exactly once")

	for offset := startOffset; offset < endOffset; offset++ {
		value, ok := consumedOffsets[offset]
		assert.True(t, ok, "consumer should observe offset %d from the produced range", offset)
		assert.Equal(t, payload, value, "produced message payload should match at offset %d", offset)
	}

	observability.Shutdown(t)

	logData, err := os.ReadFile(observability.logPath)
	assert.NoError(t, err, "consumer log file should be readable")
	logText := string(logData)
	assert.Contains(t, logText, "KAFKA_CONSUME", "consumer should emit consume log lines")
	assert.Contains(t, logText, "consumer-msg-0", "consumer logs should include the freshly produced kafka key")
	assert.Contains(t, logText, "_topic", "consumer logs should include topic field")
	assert.Contains(t, logText, "_consumer_group", "consumer logs should include consumer group field")
	assert.Contains(t, logText, brokerAddr, "consumer logs should include the configured broker endpoint")
	assert.Contains(t, logText, "_trace_id", "consumer logs should include trace id field")
	assert.Contains(t, logText, "_span_id", "consumer logs should include span id field")
	assert.Contains(t, logText, "_parent_span_id", "consumer logs should include parent span id field when trace context is propagated")

	if traceCollector != nil {
		spans := traceCollector.waitForSpans(t, producedCount*2, 5*time.Second)
		consumerSpans := filterSpansByNameAndOffsetRange(spans, "kafka.consume "+handlerName, startOffset, endOffset)

		for _, span := range consumerSpans {
			assert.NotEmpty(t, hex.EncodeToString(span.TraceId), "consumer span should carry a trace id")
			assert.NotEmpty(t, hex.EncodeToString(span.ParentSpanId), "consumer span should link to the producer span through propagated context")
			assert.Equal(t, tracepb.Span_SPAN_KIND_CONSUMER, span.Kind, "consumer span kind should be consumer")
		}
	}
}

type consumedMessageWindow struct {
	mu       sync.Mutex
	payloads map[int64]string
	err      error
}

func (w *consumedMessageWindow) record(startOffset, endOffset int64, expectedPayload string, msg *sarama.ConsumerMessage) {
	if msg.Offset < startOffset || msg.Offset >= endOffset {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	value := string(msg.Value)
	if value != expectedPayload && w.err == nil {
		w.err = fmt.Errorf("unexpected payload at offset %d: %q", msg.Offset, value)
	}
	w.payloads[msg.Offset] = value
}

func (w *consumedMessageWindow) snapshot() (error, map[int64]string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	payloads := make(map[int64]string, len(w.payloads))
	for offset, value := range w.payloads {
		payloads[offset] = value
	}
	return w.err, payloads
}

func filterSpansByNameAndOffsetRange(
	spans []*tracepb.Span,
	name string,
	startOffset int64,
	endOffset int64,
) []*tracepb.Span {
	filtered := make([]*tracepb.Span, 0, len(spans))
	for _, span := range spans {
		if span.Name != name {
			continue
		}
		offset, ok := spanInt64Attribute(span, "kafka.offset")
		if !ok {
			continue
		}
		if offset >= startOffset && offset < endOffset {
			filtered = append(filtered, span)
		}
	}
	return filtered
}

func spanInt64Attribute(span *tracepb.Span, key string) (int64, bool) {
	for _, attribute := range span.Attributes {
		if attribute.Key != key {
			continue
		}
		value := attribute.GetValue()
		if value == nil {
			return 0, false
		}
		return value.GetIntValue(), true
	}
	return 0, false
}
