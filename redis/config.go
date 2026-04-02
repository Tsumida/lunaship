package redis

import (
	"strings"

	"github.com/tsumida/lunaship/configparse"
)

type AppConfig struct {
	Instances map[string]InstanceConfig `toml:",remain"`
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

func (c AppConfig) Validate(problems *configparse.Problems) {
	for name, instance := range c.Instances {
		if strings.TrimSpace(instance.Addr) == "" {
			problems.Add("redis."+name+".addr", "is required")
		}
		configparse.ValidatePort(instance.Port, "redis."+name+".port", problems)
		if instance.DB < 0 {
			problems.Add("redis."+name+".db", "must be greater than or equal to 0")
		}
	}
}
