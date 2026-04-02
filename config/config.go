package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tsumida/lunaship/configparse"
	"github.com/tsumida/lunaship/kafka"
	"github.com/tsumida/lunaship/mysql"
	lunaredis "github.com/tsumida/lunaship/redis"
)

type AppConfig struct {
	App   AppSection          `toml:"app"`
	Redis lunaredis.AppConfig `toml:"redis"`
	MySQL mysql.MySQLConfig   `toml:"mysql"`
	Kafka kafka.KafkaConfig   `toml:"kafka"`

	raw map[string]any
}

func Load(path string) (*AppConfig, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, &LoadError{
			Kind:   ErrorKindRead,
			Source: path,
			Err:    err,
		}
	}

	return LoadFromBytes(body)
}

func LoadFromBytes(body []byte) (*AppConfig, error) {
	var raw map[string]any
	if err := toml.Unmarshal(body, &raw); err != nil {
		return nil, &LoadError{
			Kind: ErrorKindParse,
			Err:  err,
		}
	}
	if raw == nil {
		raw = map[string]any{}
	}

	cfg := &AppConfig{
		App:   defaultAppSection(),
		Redis: lunaredis.DefaultAppConfig(),
		MySQL: mysql.DefaultMySQLConfig(),
		Kafka: kafka.DefaultKafkaConfig(),
		raw:   raw,
	}

	problems := &configparse.Problems{}

	appTable, ok := configparse.RequireTable(raw, "app", "app", problems)
	if ok {
		cfg.App = decodeAppSection(appTable, problems)
	}

	if redisTable, ok := configparse.OptionalTable(raw, "redis", "redis", problems); ok {
		cfg.Redis = lunaredis.DecodeAppConfig(redisTable, problems)
	}
	if mysqlTable, ok := configparse.OptionalTable(raw, "mysql", "mysql", problems); ok {
		cfg.MySQL = mysql.DecodeMySQLConfig(mysqlTable, problems)
	}
	if kafkaTable, ok := configparse.OptionalTable(raw, "kafka", "kafka", problems); ok {
		cfg.Kafka = kafka.DecodeKafkaConfig(kafkaTable, problems)
	}

	if problems.HasErrors() {
		return nil, &LoadError{
			Kind:    ErrorKindValidation,
			Details: problems.Errors(),
		}
	}

	return cfg, nil
}

func (c *AppConfig) GetCustomConfig(section string, out any) error {
	if c == nil {
		return fmt.Errorf("app config is nil")
	}
	if err := requireDecodeTarget(out); err != nil {
		return err
	}

	value, err := lookupSection(c.raw, section)
	if err != nil {
		return err
	}

	body, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal custom config %q: %w", section, err)
	}
	if err := toml.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return fmt.Errorf("decode custom config %q: %w", section, err)
	}
	return nil
}

func requireDecodeTarget(out any) error {
	if out == nil {
		return fmt.Errorf("decode target must be a non-nil pointer")
	}

	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("decode target must be a non-nil pointer")
	}
	return nil
}

func lookupSection(raw map[string]any, section string) (any, error) {
	normalized := strings.TrimSpace(section)
	normalized = strings.TrimPrefix(normalized, ".")
	if normalized == "" {
		return nil, fmt.Errorf("custom config section must not be empty")
	}

	parts := strings.Split(normalized, ".")
	current := any(raw)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid custom config section %q", section)
		}

		table, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("custom config section %q is not a table path", section)
		}

		next, ok := table[part]
		if !ok {
			return nil, fmt.Errorf("custom config section %q not found", section)
		}
		current = next
	}

	return current, nil
}
