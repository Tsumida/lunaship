package redis

import (
	"strings"

	"github.com/tsumida/lunaship/configparse"
)

type AppConfig struct {
	Instances map[string]InstanceConfig
}

type InstanceConfig struct {
	Addr     string `toml:"addr"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		Instances: map[string]InstanceConfig{},
	}
}

func DecodeAppConfig(raw map[string]any, problems *configparse.Problems) AppConfig {
	cfg := DefaultAppConfig()

	for name, value := range raw {
		table, ok := configparse.AsTable(value)
		if !ok {
			problems.Add("redis."+name, "must be a table")
			continue
		}

		instance := InstanceConfig{}
		for key := range table {
			switch key {
			case "addr", "port", "password", "db":
			default:
				problems.Add("redis."+name+"."+key, "unknown field")
			}
		}
		if value, ok := configparse.OptionalString(table, "addr", "redis."+name+".addr", problems); ok {
			instance.Addr = strings.TrimSpace(value)
		}
		if value, ok := configparse.OptionalInt(table, "port", "redis."+name+".port", problems); ok {
			instance.Port = value
		}
		if value, ok := configparse.OptionalString(table, "password", "redis."+name+".password", problems); ok {
			instance.Password = value
		}
		if value, ok := configparse.OptionalInt(table, "db", "redis."+name+".db", problems); ok {
			instance.DB = value
		}

		if strings.TrimSpace(instance.Addr) == "" {
			problems.Add("redis."+name+".addr", "is required")
		}
		configparse.ValidatePort(instance.Port, "redis."+name+".port", problems)
		if instance.DB < 0 {
			problems.Add("redis."+name+".db", "must be greater than or equal to 0")
		}

		cfg.Instances[name] = instance
	}

	return cfg
}
