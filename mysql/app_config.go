package mysql

import (
	"strings"
	"time"

	"github.com/tsumida/lunaship/configparse"
)

const (
	defaultMySQLMaxOpenConn = 200
	defaultMySQLMaxIdleConn = 100
)

type MySQLConfig struct {
	MaxOpenConns int    `toml:"max_open_conns"`
	MaxIdleConns int    `toml:"max_idle_conns"`
	MaxIdleTime  string `toml:"max_idle_time"`
	Instances    map[string]InstanceConfig
}

type InstanceConfig struct {
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	Username    string `toml:"username"`
	Password    string `toml:"password"`
	Database    string `toml:"database"`
	PingEnabled bool   `toml:"ping_enabled"`
}

func DefaultMySQLConfig() MySQLConfig {
	return MySQLConfig{
		MaxOpenConns: defaultMySQLMaxOpenConn,
		MaxIdleConns: defaultMySQLMaxIdleConn,
		Instances:    map[string]InstanceConfig{},
	}
}

func DecodeMySQLConfig(raw map[string]any, problems *configparse.Problems) MySQLConfig {
	cfg := DefaultMySQLConfig()

	for key, value := range raw {
		switch key {
		case "max_open_conns":
			if parsed, ok := configparse.OptionalInt(raw, key, "mysql.max_open_conns", problems); ok {
				cfg.MaxOpenConns = parsed
			}
		case "max_idle_conns":
			if parsed, ok := configparse.OptionalInt(raw, key, "mysql.max_idle_conns", problems); ok {
				cfg.MaxIdleConns = parsed
			}
		case "max_idle_time":
			if parsed, ok := configparse.OptionalString(raw, key, "mysql.max_idle_time", problems); ok {
				cfg.MaxIdleTime = strings.TrimSpace(parsed)
			}
		default:
			table, ok := configparse.AsTable(value)
			if !ok {
				problems.Add("mysql."+key, "must be a table or a known mysql setting")
				continue
			}

			instance := InstanceConfig{}
			for field := range table {
				switch field {
				case "host", "port", "username", "password", "database", "ping_enabled":
				default:
					problems.Add("mysql."+key+"."+field, "unknown field")
				}
			}
			if parsed, ok := configparse.OptionalString(table, "host", "mysql."+key+".host", problems); ok {
				instance.Host = strings.TrimSpace(parsed)
			}
			if parsed, ok := configparse.OptionalInt(table, "port", "mysql."+key+".port", problems); ok {
				instance.Port = parsed
			}
			if parsed, ok := configparse.OptionalString(table, "username", "mysql."+key+".username", problems); ok {
				instance.Username = strings.TrimSpace(parsed)
			}
			if parsed, ok := configparse.OptionalString(table, "password", "mysql."+key+".password", problems); ok {
				instance.Password = parsed
			}
			if parsed, ok := configparse.OptionalString(table, "database", "mysql."+key+".database", problems); ok {
				instance.Database = strings.TrimSpace(parsed)
			}
			if parsed, ok := configparse.OptionalBool(table, "ping_enabled", "mysql."+key+".ping_enabled", problems); ok {
				instance.PingEnabled = parsed
			}

			if strings.TrimSpace(instance.Host) == "" {
				problems.Add("mysql."+key+".host", "is required")
			}
			configparse.ValidatePort(instance.Port, "mysql."+key+".port", problems)
			if strings.TrimSpace(instance.Username) == "" {
				problems.Add("mysql."+key+".username", "is required")
			}
			if strings.TrimSpace(instance.Database) == "" {
				problems.Add("mysql."+key+".database", "is required")
			}

			cfg.Instances[key] = instance
		}
	}

	if cfg.MaxOpenConns <= 0 {
		problems.Add("mysql.max_open_conns", "must be greater than 0")
	}
	if cfg.MaxIdleConns < 0 {
		problems.Add("mysql.max_idle_conns", "must be greater than or equal to 0")
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		problems.Add("mysql.max_idle_conns", "must be less than or equal to mysql.max_open_conns")
	}
	if cfg.MaxIdleTime != "" {
		if _, err := time.ParseDuration(cfg.MaxIdleTime); err != nil {
			problems.Add("mysql.max_idle_time", "must be a valid Go duration string")
		}
	}

	return cfg
}
