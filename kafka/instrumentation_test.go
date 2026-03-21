package kafka

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
)

// Description:
// Build consumer message metadata with a configured broker list.
//
// Expectation:
// The metadata keeps the logical consumer name while using the first broker endpoint
// for instance-related fields.
func TestBuildConsumerMessageMetadata_UsesConfiguredBrokerEndpoint(t *testing.T) {
	message := &sarama.ConsumerMessage{
		Topic:     "orders.created",
		Partition: 2,
		Offset:    42,
		Key:       []byte("order-42"),
	}

	metadata := buildConsumerMessageMetadata(
		"integration-consumer",
		"billing-worker",
		" kafka-1.internal:9092 , kafka-2.internal:9092 ",
		message,
	)

	assert.Equal(t, "integration-consumer", metadata.consumerName, "consumer name should keep the logical handler name")
	assert.Equal(t, "billing-worker", metadata.consumerGroup, "consumer group should be preserved")
	assert.Equal(t, "kafka-1.internal:9092", metadata.instanceAddr, "instance address should use the first configured broker endpoint")
	assert.Equal(t, "kafka-1.internal:9092", metadata.kafkaInstance, "kafka instance label should match the resolved broker endpoint")
	assert.Equal(t, "order-42", metadata.kafkaKey, "printable message keys should be preserved")
}

// Description:
// Resolve the primary broker endpoint from a broker list containing empty entries.
//
// Expectation:
// The helper skips blank entries and returns the first non-empty broker endpoint.
func TestPrimaryBrokerEndpoint_SkipsEmptyEntries(t *testing.T) {
	endpoint := primaryBrokerEndpoint(" ,  , kafka:9092 , kafka-backup:9092 ")

	assert.Equal(t, "kafka:9092", endpoint, "broker endpoint resolution should skip blank entries")
}

// Description:
// Build producer message metadata with a configured broker list.
//
// Expectation:
// The metadata uses the first broker endpoint for instance-related fields and
// preserves printable keys.
func TestBuildProducerMessageMetadata_UsesConfiguredBrokerEndpoint(t *testing.T) {
	metadata := buildProducerMessageMetadata(
		" kafka-1.internal:9092 , kafka-2.internal:9092 ",
		"orders.created",
		[]byte("order-42"),
	)

	assert.Equal(t, "orders.created", metadata.topic, "topic should be preserved")
	assert.Equal(t, "kafka-1.internal:9092", metadata.instanceAddr, "instance address should use the first configured broker endpoint")
	assert.Equal(t, "kafka-1.internal:9092", metadata.kafkaInstance, "kafka instance label should match the resolved broker endpoint")
	assert.Equal(t, "order-42", metadata.kafkaKey, "printable message keys should be preserved")
}

// Description:
// Inject trace context into sarama producer headers.
//
// Expectation:
// The header carrier should append missing headers when Set is called.
func TestSaramaProducerHeaderCarrier_AppendsMissingHeader(t *testing.T) {
	headers := make([]sarama.RecordHeader, 0)
	carrier := saramaProducerHeaderCarrier{headers: &headers}

	carrier.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")

	assert.Len(t, headers, 1, "carrier should append a new header when key is missing")
	assert.Equal(t, "traceparent", string(headers[0].Key), "header key should match")
	assert.Contains(t, string(headers[0].Value), "00-", "header value should be stored")
}
