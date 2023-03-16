package server

import (
	"os"
	"strconv"

	"github.com/tsumida/lunaship/utils"
)

type RedisConfig struct{}

type MySQLConfig struct {
	Host          string
	Port          string
	Database      string
	User          string
	Pwd           string
	MaxConnectCnt uint
}

func (c *MySQLConfig) LoadFromEnv() error {
	c.Host = utils.StrOrDefault(os.Getenv("MYSQL_HOST"), "localhost")
	c.Port = utils.StrOrDefault(os.Getenv("MYSQL_PORT"), "3306")
	c.Database = utils.StrOrDefault(os.Getenv("MYSQL_DB"), "")
	c.User = utils.StrOrDefault(os.Getenv("MYSQL_USER"), "root")
	c.Pwd = utils.StrOrDefault(os.Getenv("MYSQL_PORT"), "123456")

	if maxConnectCnt, err := strconv.Atoi(os.Getenv("MYSQL_MAX_CONN")); err == nil {
		c.MaxConnectCnt = uint(maxConnectCnt)
	}

	return nil
}

type ResourceInfo struct {
	MySQL *MySQLConfig
	Redis *RedisConfig
}

const (
	ENV_LIVE = "live"
	ENV_TEST = "test"
	ENV_DEV  = "dev"
)

type ServerConfig struct {
	ServiceId         string
	ServiceVersion    string
	ServiceOpenapiDoc string
	DeployEnv         string
}

func (sc *ServerConfig) LoadFromEnv() error {
	sc.ServiceId = os.Getenv("SERVICE_ID")
	sc.ServiceVersion = os.Getenv("SERVICE_VERSION")
	sc.ServiceOpenapiDoc = os.Getenv("SERVICE_DOC")

	return nil
}
