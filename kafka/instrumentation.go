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
const kafkaProduceLogMessage = "KAFKA_PRODUCE"

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

type producerMessageMetadata struct {
	topic         string
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

type saramaProducerHeaderCarrier struct {
	headers *[]sarama.RecordHeader
}

func (carrier saramaProducerHeaderCarrier) Get(key string) string {
	for _, header := range *carrier.headers {
		if strings.EqualFold(string(header.Key), key) {
			return string(header.Value)
		}
	}
	return ""
}

func (carrier saramaProducerHeaderCarrier) Set(key, value string) {
	for i := range *carrier.headers {
		if strings.EqualFold(string((*carrier.headers)[i].Key), key) {
			(*carrier.headers)[i].Value = []byte(value)
			return
		}
	}
	*carrier.headers = append(*carrier.headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (carrier saramaProducerHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*carrier.headers))
	for _, header := range *carrier.headers {
		keys = append(keys, string(header.Key))
	}
	return keys
}

func startConsumerInstrumentation(
	baseCtx context.Context,
	consumerName string,
	consumerGroup string,
	brokerAddr string,
	message *sarama.ConsumerMessage,
) (context.Context, trace.Span, consumerMessageMetadata) {
	metadata := buildConsumerMessageMetadata(consumerName, consumerGroup, brokerAddr, message)
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

func startProducerInstrumentation(
	baseCtx context.Context,
	brokers string,
	topic string,
	key []byte,
	headers *[]sarama.RecordHeader,
) (context.Context, trace.Span, producerMessageMetadata) {
	metadata := buildProducerMessageMetadata(brokers, topic, key)
	parentSpanID := parentSpanIDFromContext(baseCtx)

	ctx, span := otel.Tracer(kafkaTracerName()).Start(
		baseCtx,
		producerSpanName(topic),
		trace.WithSpanKind(trace.SpanKindProducer),
	)
	span.SetAttributes(producerSpanAttributes(metadata)...)

	traceID, spanID, sampled := traceIdentifiers(span.SpanContext())
	ctx = log.WithTrace(ctx, traceID, spanID, parentSpanID, sampled)
	ctx = log.WithFields(ctx, producerContextFields(metadata)...)

	// Propagate the producer span context to downstream consumers.
	otel.GetTextMapPropagator().Inject(ctx, saramaProducerHeaderCarrier{headers: headers})
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

func finishProducerInstrumentation(
	ctx context.Context,
	span trace.Span,
	metadata producerMessageMetadata,
	partition int32,
	offset int64,
	err error,
) {
	durationMs := time.Since(metadata.startedAt).Milliseconds()
	span.SetAttributes(
		attribute.Int64("kafka.partition", int64(partition)),
		attribute.Int64("kafka.offset", offset),
		attribute.Bool("error.flag", err != nil),
		attribute.Int64("duration.ms", durationMs),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	if err != nil {
		log.Logger(ctx).Error(
			kafkaProduceLogMessage,
			zap.Int64("_dur_ms", durationMs),
			zap.String("_error", err.Error()),
		)
		return
	}

	log.Logger(ctx).Info(
		kafkaProduceLogMessage,
		zap.Int32("_partition", partition),
		zap.Int64("_offset", offset),
		zap.Int64("_dur_ms", durationMs),
	)
}

func buildConsumerMessageMetadata(
	consumerName string,
	consumerGroup string,
	brokerAddr string,
	message *sarama.ConsumerMessage,
) consumerMessageMetadata {
	metadata := consumerMessageMetadata{
		topic:         message.Topic,
		partition:     message.Partition,
		offset:        message.Offset,
		consumerGroup: strings.TrimSpace(consumerGroup),
		consumerName:  strings.TrimSpace(consumerName),
		instanceAddr:  primaryBrokerEndpoint(brokerAddr),
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

func buildProducerMessageMetadata(brokers string, topic string, key []byte) producerMessageMetadata {
	metadata := producerMessageMetadata{
		topic:        strings.TrimSpace(topic),
		instanceAddr: primaryBrokerEndpoint(brokers),
		instancePort: 0,
		appName:      kafkaAppName(),
		startedAt:    time.Now(),
	}
	if metadata.instanceAddr != "" {
		metadata.kafkaInstance = joinHostPort(metadata.instanceAddr, metadata.instancePort)
	}
	if isPrintableKafkaKey(key) {
		metadata.kafkaKey = string(key)
	}
	return metadata
}

func primaryBrokerEndpoint(brokers string) string {
	for _, broker := range strings.Split(brokers, ",") {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			return broker
		}
	}
	return ""
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
	if metadata.instancePort > 0 {
		fields = append(fields, zap.Int("_instance_port", metadata.instancePort))
	}
	if metadata.kafkaKey != "" {
		fields = append(fields, zap.String("_kafka_key", metadata.kafkaKey))
	}
	return fields
}

func producerContextFields(metadata producerMessageMetadata) []zap.Field {
	fields := []zap.Field{
		zap.String("_topic", metadata.topic),
		zap.String("_instance_addr", metadata.instanceAddr),
	}
	if metadata.instancePort > 0 {
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

func producerSpanAttributes(metadata producerMessageMetadata) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String("kafka.topic", metadata.topic),
		attribute.String("kafka.op", "produce"),
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

func producerSpanName(topic string) string {
	return fmt.Sprintf("kafka.produce %s", strings.TrimSpace(topic))
}

func kafkaTracerName() string {
	name := strings.TrimSpace(os.Getenv("APP_NAME"))
	if name == "" {
		return "lunaship.kafka"
	}
	return name
}

func consumerTracerName() string {
	return kafkaTracerName()
}

func kafkaAppName() string {
	if app, _, _ := log.AppIdentity(); strings.TrimSpace(app) != "" {
		return strings.TrimSpace(app)
	}
	return strings.TrimSpace(os.Getenv("APP_NAME"))
}

func consumerAppName() string {
	return kafkaAppName()
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
