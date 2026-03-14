package kafka

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IBM/sarama"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const kafkaConsumeLogMessage = "KAFKA_CONSUME"

type consumerMessageMetadata struct {
	topic         string
	partition     int32
	offset        int64
	consumerGroup string
	consumerName  string
	instanceAddr  string
	instancePort  int
	kafkaKey      string
	appName       string
	kafkaInstance string
	startedAt     time.Time
}

type saramaHeaderCarrier []*sarama.RecordHeader

func (carrier saramaHeaderCarrier) Get(key string) string {
	for _, header := range carrier {
		if strings.EqualFold(string(header.Key), key) {
			return string(header.Value)
		}
	}
	return ""
}

func (carrier saramaHeaderCarrier) Set(key, value string) {
	for _, header := range carrier {
		if strings.EqualFold(string(header.Key), key) {
			header.Value = []byte(value)
			return
		}
	}
}

func (carrier saramaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(carrier))
	for _, header := range carrier {
		keys = append(keys, string(header.Key))
	}
	return keys
}

func startConsumerInstrumentation(
	baseCtx context.Context,
	consumerName string,
	consumerGroup string,
	message *sarama.ConsumerMessage,
) (context.Context, trace.Span, consumerMessageMetadata) {
	metadata := buildConsumerMessageMetadata(consumerName, consumerGroup, message)
	extractedCtx := otel.GetTextMapPropagator().Extract(baseCtx, saramaHeaderCarrier(message.Headers))
	parentSpanID := parentSpanIDFromContext(extractedCtx)

	ctx, span := otel.Tracer(consumerTracerName()).Start(
		extractedCtx,
		consumerSpanName(consumerName, message.Topic),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	span.SetAttributes(consumerSpanAttributes(metadata)...)

	traceID, spanID, sampled := traceIdentifiers(span.SpanContext())
	ctx = log.WithTrace(ctx, traceID, spanID, parentSpanID, sampled)
	ctx = log.WithFields(ctx, consumerContextFields(metadata)...)
	return ctx, span, metadata
}

func finishConsumerInstrumentation(
	ctx context.Context,
	span trace.Span,
	err error,
	duration time.Duration,
) {
	durationMs := duration.Milliseconds()
	span.SetAttributes(
		attribute.Bool("error.flag", err != nil),
		attribute.Int64("duration.ms", durationMs),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	fields := []zap.Field{zap.Int64("_dur_ms", durationMs)}
	if err != nil {
		fields = append(fields, zap.String("_error", err.Error()))
		log.Logger(ctx).Error(kafkaConsumeLogMessage, fields...)
		return
	}
	log.Logger(ctx).Info(kafkaConsumeLogMessage, fields...)
}

func buildConsumerMessageMetadata(
	consumerName string,
	consumerGroup string,
	message *sarama.ConsumerMessage,
) consumerMessageMetadata {
	metadata := consumerMessageMetadata{
		topic:         message.Topic,
		partition:     message.Partition,
		offset:        message.Offset,
		consumerGroup: strings.TrimSpace(consumerGroup),
		consumerName:  strings.TrimSpace(consumerName),
		instanceAddr:  "",
		instancePort:  0,
		appName:       consumerAppName(),
		startedAt:     time.Now(),
	}
	if metadata.instanceAddr != "" {
		metadata.kafkaInstance = joinHostPort(metadata.instanceAddr, metadata.instancePort)
	}
	if isPrintableKafkaKey(message.Key) {
		metadata.kafkaKey = string(message.Key)
	}
	return metadata
}

func consumerContextFields(metadata consumerMessageMetadata) []zap.Field {
	fields := []zap.Field{
		zap.String("_topic", metadata.topic),
		zap.Int32("_partition", metadata.partition),
		zap.Int64("_offset", metadata.offset),
		zap.String("_consumer_group", metadata.consumerGroup),
		zap.String("_instance_addr", metadata.instanceAddr),
	}
	if metadata.consumerName != "" {
		fields = append(fields, zap.String("_consumer", metadata.consumerName))
	}
	if metadata.instanceAddr != "" {
		fields = append(fields, zap.Int("_instance_port", metadata.instancePort))
	}
	if metadata.kafkaKey != "" {
		fields = append(fields, zap.String("_kafka_key", metadata.kafkaKey))
	}
	return fields
}

func consumerSpanAttributes(metadata consumerMessageMetadata) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String("kafka.topic", metadata.topic),
		attribute.Int64("kafka.partition", int64(metadata.partition)),
		attribute.String("kafka.op", "consume"),
		attribute.Int64("kafka.offset", metadata.offset),
		attribute.String("consumer.group", metadata.consumerGroup),
	}
	if metadata.kafkaInstance != "" {
		attributes = append(attributes, attribute.String("instance", metadata.kafkaInstance))
	}
	return attributes
}

func consumerSpanName(consumerName, topic string) string {
	if strings.TrimSpace(consumerName) != "" {
		return fmt.Sprintf("kafka.consume %s", consumerName)
	}
	return fmt.Sprintf("kafka.consume %s", topic)
}

func consumerTracerName() string {
	name := strings.TrimSpace(os.Getenv("APP_NAME"))
	if name == "" {
		return "lunaship.kafka"
	}
	return name
}

func consumerAppName() string {
	if app, _, _ := log.AppIdentity(); strings.TrimSpace(app) != "" {
		return strings.TrimSpace(app)
	}
	return strings.TrimSpace(os.Getenv("APP_NAME"))
}

func joinHostPort(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port <= 0 {
		return host
	}
	return host + ":" + strconv.Itoa(port)
}

func traceIdentifiers(sc trace.SpanContext) (traceID, spanID string, sampled bool) {
	if !sc.IsValid() {
		return "", "", true
	}
	return sc.TraceID().String(), sc.SpanID().String(), sc.TraceFlags().IsSampled()
}

func parentSpanIDFromContext(ctx context.Context) string {
	parent := trace.SpanContextFromContext(ctx)
	if !parent.IsValid() {
		return ""
	}
	return parent.SpanID().String()
}

func isPrintableKafkaKey(key []byte) bool {
	if len(key) == 0 || !utf8.Valid(key) {
		return false
	}
	for _, r := range string(key) {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
