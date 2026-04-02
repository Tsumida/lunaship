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
	MaxOpenConns int                       `toml:"max_open_conns"`
	MaxIdleConns int                       `toml:"max_idle_conns"`
	MaxIdleTime  time.Duration             `toml:"max_idle_time"`
	Instances    map[string]InstanceConfig `toml:",remain"`
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

func (c MySQLConfig) Validate(problems *configparse.Problems) {
	if c.MaxOpenConns <= 0 {
		problems.Add("mysql.max_open_conns", "must be greater than 0")
	}
	if c.MaxIdleConns < 0 {
		problems.Add("mysql.max_idle_conns", "must be greater than or equal to 0")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		problems.Add("mysql.max_idle_conns", "must be less than or equal to mysql.max_open_conns")
	}
	configparse.ValidateDuration(c.MaxIdleTime, "mysql.max_idle_time", problems)

	for name, instance := range c.Instances {
		if strings.TrimSpace(instance.Host) == "" {
			problems.Add("mysql."+name+".host", "is required")
		}
		configparse.ValidatePort(instance.Port, "mysql."+name+".port", problems)
		if strings.TrimSpace(instance.Username) == "" {
			problems.Add("mysql."+name+".username", "is required")
		}
		if strings.TrimSpace(instance.Database) == "" {
			problems.Add("mysql."+name+".database", "is required")
		}
	}
}
