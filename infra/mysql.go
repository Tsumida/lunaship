package infra

import (
	"fmt"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	_global_mysql *gorm.DB = nil
	_init_mysql            = sync.Once{}
)

func GlobalMySQL() *gorm.DB {
	return _global_mysql
}

func LoadMySQLConfFromEnv(debug bool) mysql.Config {
	var (
		usr    = utils.StrOrDefault(os.Getenv("MYSQL_USER"), "root")
		pwd    = utils.StrOrDefault(os.Getenv("MYSQL_PASSWORD"), "helloworld")
		host   = utils.StrOrDefault(os.Getenv("MYSQL_HOST"), "192.168.0.120")
		port   = utils.StrOrDefault(os.Getenv("MYSQL_PORT"), "3306")
		dbName = utils.StrOrDefault(os.Getenv("MYSQL_DATABASE"), "metaserver")
	)

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		usr, pwd, host, port, dbName,
	)
	if debug {
		log.GlobalLog().Info("mysql conf read", zap.String("dsn", dsn))
	}
	return mysql.Config{
		DSN: dsn,
	}
}

func InitMySQL(
	conf mysql.Config,
	gormConf gorm.Config,
	migrateFunc func(db *gorm.DB) error,
) (err error) {
	_init_mysql.Do(func() {

		db, e := gorm.Open(
			mysql.New(conf),
			&gormConf,
		)
		if e != nil {
			err = e
			return
		}
		if db == nil {
			err = fmt.Errorf("mysql init failed")
			return
		}
		_global_mysql = db

		sqlDB, e := db.DB()
		if e != nil {
			err = errors.WithMessage(e, "config db")
			return
		}

		sqlDB.SetMaxIdleConns(100)
		sqlDB.SetMaxOpenConns(200)

		if e := migrateFunc(_global_mysql); e != nil {
			err = e
		}

		log.GlobalLog().Info("mysql connected")
	})

	return err
}
