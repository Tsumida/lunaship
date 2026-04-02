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
	Producer map[string]ProducerConfig
	Consumer map[string]ConsumerConfig
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

func DecodeKafkaConfig(raw map[string]any, problems *configparse.Problems) KafkaConfig {
	cfg := DefaultKafkaConfig()

	for key := range raw {
		switch key {
		case "producer", "consumer":
		default:
			problems.Add("kafka."+key, "unknown field")
		}
	}

	if table, ok := configparse.OptionalTable(raw, "producer", "kafka.producer", problems); ok {
		for name, value := range table {
			instanceTable, ok := configparse.AsTable(value)
			if !ok {
				problems.Add("kafka.producer."+name, "must be a table")
				continue
			}

			instance := ProducerConfig{
				Acks: defaultKafkaAcks,
			}
			for key := range instanceTable {
				switch key {
				case "brokers", "topic", "acks":
				default:
					problems.Add("kafka.producer."+name+"."+key, "unknown field")
				}
			}

			if value, ok := configparse.OptionalStringSlice(instanceTable, "brokers", "kafka.producer."+name+".brokers", problems); ok {
				instance.Brokers = configparse.TrimStringSlice(value)
			}
			if value, ok := configparse.OptionalString(instanceTable, "topic", "kafka.producer."+name+".topic", problems); ok {
				instance.Topic = strings.TrimSpace(value)
			}
			if value, ok := configparse.OptionalString(instanceTable, "acks", "kafka.producer."+name+".acks", problems); ok {
				instance.Acks = strings.ToLower(strings.TrimSpace(value))
			}

			validateBrokers(instance.Brokers, "kafka.producer."+name+".brokers", problems)
			if strings.TrimSpace(instance.Topic) == "" {
				problems.Add("kafka.producer."+name+".topic", "is required")
			}
			if _, ok := validKafkaAcks[instance.Acks]; !ok {
				problems.Add("kafka.producer."+name+".acks", fmt.Sprintf("must be one of %q", configparse.OrderedKeys(validKafkaAcks)))
			}

			cfg.Producer[name] = instance
		}
	}

	if table, ok := configparse.OptionalTable(raw, "consumer", "kafka.consumer", problems); ok {
		for name, value := range table {
			instanceTable, ok := configparse.AsTable(value)
			if !ok {
				problems.Add("kafka.consumer."+name, "must be a table")
				continue
			}

			instance := ConsumerConfig{}
			for key := range instanceTable {
				switch key {
				case "brokers", "topic", "consumer_group":
				default:
					problems.Add("kafka.consumer."+name+"."+key, "unknown field")
				}
			}

			if value, ok := configparse.OptionalStringSlice(instanceTable, "brokers", "kafka.consumer."+name+".brokers", problems); ok {
				instance.Brokers = configparse.TrimStringSlice(value)
			}
			if value, ok := configparse.OptionalString(instanceTable, "topic", "kafka.consumer."+name+".topic", problems); ok {
				instance.Topic = strings.TrimSpace(value)
			}
			if value, ok := configparse.OptionalString(instanceTable, "consumer_group", "kafka.consumer."+name+".consumer_group", problems); ok {
				instance.ConsumerGroup = strings.TrimSpace(value)
			}

			validateBrokers(instance.Brokers, "kafka.consumer."+name+".brokers", problems)
			if strings.TrimSpace(instance.Topic) == "" {
				problems.Add("kafka.consumer."+name+".topic", "is required")
			}
			if strings.TrimSpace(instance.ConsumerGroup) == "" {
				problems.Add("kafka.consumer."+name+".consumer_group", "is required")
			}

			cfg.Consumer[name] = instance
		}
	}

	return cfg
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
