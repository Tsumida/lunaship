package kafka

import (
	"fmt"
	"strings"

	"github.com/tsumida/lunaship/configparse"
)

const defaultKafkaAcks = "all"

var validKafkaAcks = map[string]struct{}{
	"0":   {},
	"1":   {},
	"all": {},
}

type KafkaConfig struct {
	Producer map[string]ProducerConfig `toml:"producer"`
	Consumer map[string]ConsumerConfig `toml:"consumer"`
}

type ProducerConfig struct {
	Brokers []string `toml:"brokers"`
	Topic   string   `toml:"topic"`
	Acks    string   `toml:"acks"`
}

type ConsumerConfig struct {
	Brokers       []string `toml:"brokers"`
	Topic         string   `toml:"topic"`
	ConsumerGroup string   `toml:"consumer_group"`
}

func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Producer: map[string]ProducerConfig{},
		Consumer: map[string]ConsumerConfig{},
	}
}

func (c *KafkaConfig) Normalize() {
	if c == nil {
		return
	}
	if c.Producer == nil {
		c.Producer = map[string]ProducerConfig{}
	}
	for name, instance := range c.Producer {
		instance.Brokers = configparse.TrimStringSlice(instance.Brokers)
		instance.Topic = strings.TrimSpace(instance.Topic)
		instance.Acks = strings.ToLower(strings.TrimSpace(instance.Acks))
		if instance.Acks == "" {
			instance.Acks = defaultKafkaAcks
		}
		c.Producer[name] = instance
	}

	if c.Consumer == nil {
		c.Consumer = map[string]ConsumerConfig{}
	}
	for name, instance := range c.Consumer {
		instance.Brokers = configparse.TrimStringSlice(instance.Brokers)
		instance.Topic = strings.TrimSpace(instance.Topic)
		instance.ConsumerGroup = strings.TrimSpace(instance.ConsumerGroup)
		c.Consumer[name] = instance
	}
}

func (c KafkaConfig) Validate(problems *configparse.Problems) {
	for name, instance := range c.Producer {
		validateBrokers(instance.Brokers, "kafka.producer."+name+".brokers", problems)
		if strings.TrimSpace(instance.Topic) == "" {
			problems.Add("kafka.producer."+name+".topic", "is required")
		}
		if _, ok := validKafkaAcks[instance.Acks]; !ok {
			problems.Add("kafka.producer."+name+".acks", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validKafkaAcks)))
		}
	}

	for name, instance := range c.Consumer {
		validateBrokers(instance.Brokers, "kafka.consumer."+name+".brokers", problems)
		if strings.TrimSpace(instance.Topic) == "" {
			problems.Add("kafka.consumer."+name+".topic", "is required")
		}
		if strings.TrimSpace(instance.ConsumerGroup) == "" {
			problems.Add("kafka.consumer."+name+".consumer_group", "is required")
		}
	}
}

func validateBrokers(brokers []string, path string, problems *configparse.Problems) {
	if len(brokers) == 0 {
		problems.Add(path, "must contain at least one broker")
		return
	}

	for index, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			problems.Add(fmt.Sprintf("%s[%d]", path, index), "must not be empty")
		}
	}
}
