package kafka

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/IBM/sarama"
)

var (
	ErrProducerClosed = errors.New("kafka producer is closed")
	ErrNoBrokers      = errors.New("kafka producer brokers is empty")
)

type Message struct {
	// Key is optional. When present, Kafka uses it for partitioning and ordering.
	Key []byte

	// Headers are copied into Kafka headers before trace context is injected.
	Headers map[string]string

	Payload []byte
}

// Producer is a wrapper around sarama.SyncProducer, which provides a simple API and
// observability instrumentation for publishing messages to Kafka.
type Producer interface {
	Publish(ctx context.Context, msg Message) (partition int32, offset int64, err error)
	Close() error
}

type KafkaProducer struct {
	brokers   string
	topic     string
	producer  sarama.SyncProducer
	closeOnce sync.Once
	closeErr  error
}

func NewKafkaProducer(brokers string, cfg *sarama.Config, topic string) (*KafkaProducer, error) {
	brokers = strings.TrimSpace(brokers)
	if brokers == "" {
		return nil, ErrNoBrokers
	}
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("kafka producer topic is empty")
	}

	brokerList := parseBrokerList(brokers)
	if len(brokerList) == 0 {
		return nil, ErrNoBrokers
	}

	config := sarama.NewConfig()
	if cfg != nil {
		// Sarama config does not always expose a Clone() helper across versions.
		// A shallow copy is sufficient here because we only mutate a boolean flag.
		*config = *cfg
	}
	// SyncProducer requires successes to be returned.
	config.Producer.Return.Successes = true

	p, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		return nil, err
	}

	RegisterProducerMetrics()

	return &KafkaProducer{
		brokers:  brokers,
		topic:    strings.TrimSpace(topic),
		producer: p,
	}, nil
}

func (p *KafkaProducer) Publish(ctx context.Context, msg Message) (int32, int64, error) {
	if p == nil || p.producer == nil {
		return -1, -1, ErrProducerClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	message := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(msg.Payload),
	}
	if len(msg.Key) > 0 {
		message.Key = sarama.ByteEncoder(msg.Key)
	}
	if len(msg.Headers) > 0 {
		message.Headers = make([]sarama.RecordHeader, 0, len(msg.Headers))
		for k, v := range msg.Headers {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			message.Headers = append(message.Headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
	}

	instCtx, span, metadata := startProducerInstrumentation(
		ctx,
		p.brokers,
		p.topic,
		msg.Key,
		&message.Headers,
	)
	partition, offset, err := p.producer.SendMessage(message)
	finishProducerInstrumentation(instCtx, span, metadata, partition, offset, err)
	recordProducerMetrics(metadata, partition, err)
	return partition, offset, err
}

func (p *KafkaProducer) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		if p.producer == nil {
			p.closeErr = nil
			return
		}
		p.closeErr = p.producer.Close()
		p.producer = nil
	})
	return p.closeErr
}

func parseBrokerList(brokers string) []string {
	raw := strings.Split(brokers, ",")
	out := make([]string, 0, len(raw))
	for _, broker := range raw {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			continue
		}
		out = append(out, broker)
	}
	return out
}
