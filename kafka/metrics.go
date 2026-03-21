package kafka

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	kafkaConsumeDurationMs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consume_duration_ms",
			Help: "Latest Kafka consumer message handling duration in milliseconds.",
		},
		[]string{"kafka_instance", "topic", "partition", "consumer_group", "app"},
	)
	kafkaConsumeErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_consume_error_count",
			Help: "Kafka consumer message handling errors by error type.",
		},
		[]string{"kafka_instance", "topic", "partition", "consumer_group", "app", "error_type"},
	)

	kafkaProduceDurationMs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_produce_duration_ms",
			Help: "Latest Kafka producer message publish duration in milliseconds.",
		},
		[]string{"kafka_instance", "topic", "partition", "app"},
	)
	kafkaProduceErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_produce_error_count",
			Help: "Kafka producer message publish errors by error type.",
		},
		[]string{"kafka_instance", "topic", "partition", "app", "error_type"},
	)
)

func RegisterConsumerMetrics() {
	registerKafkaCollector(kafkaConsumeDurationMs)
	registerKafkaCollector(kafkaConsumeErrorCount)
}

func RegisterProducerMetrics() {
	registerKafkaCollector(kafkaProduceDurationMs)
	registerKafkaCollector(kafkaProduceErrorCount)
}

func recordConsumerMetrics(metadata consumerMessageMetadata, err error) {
	labels := []string{
		metadata.kafkaInstance,
		metadata.topic,
		strconv.FormatInt(int64(metadata.partition), 10),
		metadata.consumerGroup,
		metadata.appName,
	}

	durationMs := float64(time.Since(metadata.startedAt)) / float64(time.Millisecond)
	kafkaConsumeDurationMs.WithLabelValues(labels...).Set(durationMs)

	if err != nil {
		errorLabels := append(labels, consumerErrorType(err))
		kafkaConsumeErrorCount.WithLabelValues(errorLabels...).Inc()
	}
}

func recordProducerMetrics(metadata producerMessageMetadata, partition int32, err error) {
	labels := []string{
		metadata.kafkaInstance,
		metadata.topic,
		strconv.FormatInt(int64(partition), 10),
		metadata.appName,
	}

	durationMs := float64(time.Since(metadata.startedAt)) / float64(time.Millisecond)
	kafkaProduceDurationMs.WithLabelValues(labels...).Set(durationMs)

	if err != nil {
		errorLabels := append(labels, consumerErrorType(err))
		kafkaProduceErrorCount.WithLabelValues(errorLabels...).Inc()
	}
}

func registerKafkaCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		var alreadyRegisteredErr prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegisteredErr) {
			return
		}
		panic(err)
	}
}

func consumerErrorType(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}
