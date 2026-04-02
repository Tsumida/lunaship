package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tsumida/lunaship/configparse"
	"github.com/tsumida/lunaship/kafka"
	"github.com/tsumida/lunaship/mysql"
	lunaredis "github.com/tsumida/lunaship/redis"
)

type AppConfig struct {
	App    AppSection          `toml:"app"`
	Redis  lunaredis.AppConfig `toml:"redis"`
	MySQL  mysql.MySQLConfig   `toml:"mysql"`
	Kafka  kafka.KafkaConfig   `toml:"kafka"`
	Custom map[string]any      `toml:",remain"`

	raw map[string]any
}

var (
	globalAppConfig   *AppConfig
	globalAppConfigMu sync.RWMutex
)

func SetGlobal(cfg *AppConfig) {
	globalAppConfigMu.Lock()
	defer globalAppConfigMu.Unlock()

	globalAppConfig = cfg
}

func Global() *AppConfig {
	globalAppConfigMu.RLock()
	defer globalAppConfigMu.RUnlock()

	return globalAppConfig
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
	metadata, err := configparse.Decode(raw, cfg)
	if err != nil {
		configparse.AddDecodeError(problems, "", err)
	}
	configparse.AddUnused(problems, metadata, nil)

	cfg.App.Normalize()
	cfg.Kafka.Normalize()

	cfg.App.Validate(problems)
	cfg.Redis.Validate(problems)
	cfg.MySQL.Validate(problems)
	cfg.Kafka.Validate(problems)

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

	value, err := configparse.LookupPath(c.Custom, section)
	if err != nil {
		return fmt.Errorf("custom config section %q not found: %w", section, err)
	}

	metadata, err := configparse.Decode(value, out)
	if err != nil {
		return fmt.Errorf("decode custom config %q: %w", section, err)
	}
	if metadata != nil && len(metadata.Unused) > 0 {
		return fmt.Errorf("decode custom config %q: unknown fields: %s", section, strings.Join(metadata.Unused, ", "))
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
