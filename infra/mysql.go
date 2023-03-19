package infra

import (
	"os"
	"strconv"

	"github.com/tsumida/lunaship/infra/utils"
)

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
