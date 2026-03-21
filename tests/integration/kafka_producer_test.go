package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Description:
// Initialize observability and produce 10 messages to Kafka topic test-lunaship.
//
// Expectation:
// Every send succeeds and returns a valid partition/offset.
func TestKafkaProducer_ProducesHelloWorldMessages(t *testing.T) {
	t.Setenv("APP_NAME", "kafka-producer-integration")
	t.Setenv("SERVICE_ID", "kafka-producer-integration")
	t.Setenv("TRACE_ENABLED", "true")
	t.Setenv("TRACE_ERROR_LOG_DISABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	_ = initializeObservability(t, "producer")

	brokerAddr := envOrDefault("KAFKA_ADDR", "kafka:9092")
	topic := "test-lunaship"
	payload := "hello, world"

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3
	cfg.Net.DialTimeout = 3 * time.Second
	cfg.Net.ReadTimeout = 3 * time.Second
	cfg.Net.WriteTimeout = 3 * time.Second

	ensureTopicExists(t, []string{brokerAddr}, topic, cfg)

	producer, err := sarama.NewSyncProducer([]string{brokerAddr}, cfg)
	assert.NoError(t, err, "sync producer initialization should succeed")
	if err != nil {
		return
	}
	t.Cleanup(func() {
		_ = producer.Close()
	})

	for i := 0; i < 10; i++ {
		partition, offset, sendErr := produceWithObservability(
			context.Background(),
			producer,
			brokerAddr,
			topic,
			fmt.Sprintf("msg-%d", i),
			payload,
		)
		assert.NoError(t, sendErr, "message send should succeed")
		assert.GreaterOrEqual(t, partition, int32(0), "partition should be non-negative")
		assert.GreaterOrEqual(t, offset, int64(0), "offset should be non-negative")
	}

	time.Sleep(10 * time.Second) // wait trace report
}

func produceWithObservability(
	baseCtx context.Context,
	producer sarama.SyncProducer,
	brokerAddr string,
	topic string,
	key string,
	payload string,
) (int32, int64, error) {
	start := time.Now()
	ctx, span := otel.Tracer(kafkaTracerName()).Start(
		baseCtx,
		"kafka.produce "+topic,
		trace.WithSpanKind(trace.SpanKindProducer),
	)
	span.SetAttributes(
		attribute.String("kafka.topic", topic),
		attribute.String("kafka.op", "produce"),
		attribute.String("instance", brokerAddr),
	)

	traceCtx := span.SpanContext()
	ctx = log.WithTrace(ctx, traceCtx.TraceID().String(), traceCtx.SpanID().String(), "", traceCtx.TraceFlags().IsSampled())
	ctx = log.WithFields(
		ctx,
		zap.String("_topic", topic),
		zap.String("_instance_addr", brokerAddr),
		zap.String("_kafka_key", key),
	)

	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(payload),
	}
	otel.GetTextMapPropagator().Inject(ctx, saramaProducerHeaderCarrier{headers: &message.Headers})

	partition, offset, err := producer.SendMessage(message)

	durationMs := time.Since(start).Milliseconds()
	span.SetAttributes(
		attribute.Int64("kafka.partition", int64(partition)),
		attribute.Int64("kafka.offset", offset),
		attribute.Int64("duration.ms", durationMs),
		attribute.Bool("error.flag", err != nil),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Logger(ctx).Error("KAFKA_PRODUCE", zap.Int64("_dur_ms", durationMs), zap.String("_error", err.Error()))
		span.End()
		return partition, offset, err
	}

	log.Logger(ctx).Info(
		"KAFKA_PRODUCE",
		zap.Int32("_partition", partition),
		zap.Int64("_offset", offset),
		zap.Int64("_dur_ms", durationMs),
	)
	span.End()
	return partition, offset, nil
}
