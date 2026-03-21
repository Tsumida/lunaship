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
