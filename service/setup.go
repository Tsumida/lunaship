package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	driverMySQL "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/tsumida/lunaship/config"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/mysql"
	lunaredis "github.com/tsumida/lunaship/redis"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	ENV_LIVE = "live"
	ENV_TEST = "test"
	ENV_DEV  = "dev"

	defaultAppConfigPath   = "config/app.toml"
	defaultShutdownTimeout = 10 * time.Second
	defaultRedisTimeout    = time.Second
	defaultRedisRetry      = 2
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

type Service struct {
	Handler        http.Handler
	Path           string
	BindingAddress string
}

func (s *Service) Run(ctx context.Context) {
	cfgPath := utils.StrOrDefault(
		strings.TrimSpace(os.Getenv("APP_CONFIG_PATH")),
		defaultAppConfigPath,
	)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger := log.InitLog(
			utils.StrOrDefault(os.Getenv("LOG_FILE"), "./tmp/log.log"),
			utils.StrOrDefault(os.Getenv("ERR_FILE"), "./tmp/err.log"),
			zapcore.InfoLevel,
		)
		logger.Error("failed to load app config", zap.String("path", cfgPath), zap.Error(err))
		_ = logger.Sync()
		panic(err)
	}
	config.SetGlobal(cfg)

	applyRuntimeMetadata(cfg)

	_ = log.InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "./tmp/log.log"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "./tmp/err.log"),
		logLevelFromConfig(cfg.App.Log.Level),
	)
	defer func() {
		_ = infra.CloseTracing()
	}()
	defer func() {
		_ = log.GlobalLog().Sync()
		log.GlobalLog().Info("server done")
	}()

	if _, err := infra.InitTracing(traceConfigFromAppConfig(cfg)); err != nil {
		log.GlobalLog().Error("failed to init tracing", zap.Error(err))
	}

	if err := initRedis(ctx, cfg); err != nil {
		panic(err)
	}
	if err := initMySQL(cfg); err != nil {
		panic(err)
	}
	if err := initKafka(cfg); err != nil {
		panic(err)
	}

	var (
		registerRoutes func(mux *http.ServeMux)
		stopPprof      func()
	)
	if cfg.App.Pprof.Enabled {
		registerRoutes, stopPprof = initPprofFromConfig(cfg)
	}

	s.serve(ctx, defaultShutdownTimeout, registerRoutes, stopPprof)
}

func (s *Service) serve(
	ctx context.Context,
	shutdownDur time.Duration,
	registerRoutes func(mux *http.ServeMux),
	stopFn func(),
) {
	mux := http.NewServeMux()
	mux.Handle(s.Path, s.Handler)
	if registerRoutes != nil {
		registerRoutes(mux)
	}
	log.GlobalLog().Info(
		"server up", zap.String("listen", s.BindingAddress),
	)

	server := &http.Server{
		Addr:    s.BindingAddress,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go utils.Go(func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.GlobalLog().Error("server down", zap.Error(err))
			} else {
				log.GlobalLog().Info("server graceful shutdown")
			}
		}
	})

	<-ctx.Done()
	log.GlobalLog().Info(
		"shutting down server",
		zap.Float64("waiting_sec", shutdownDur.Seconds()),
	)

	sctx, cancel := context.WithTimeout(context.Background(), shutdownDur)
	defer cancel()

	if err := server.Shutdown(sctx); err != nil {
		log.GlobalLog().Error("failed to shutdown", zap.Error(err))
	}
	if stopFn != nil {
		stopFn()
	}
}

func applyRuntimeMetadata(cfg *config.AppConfig) {
	if cfg == nil {
		return
	}

	_ = os.Setenv("APP_NAME", cfg.App.AppName)
	if strings.TrimSpace(os.Getenv("SERVICE_ID")) == "" {
		_ = os.Setenv("SERVICE_ID", cfg.App.AppName)
	}
	if strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")) == "" {
		_ = os.Setenv("OTEL_SERVICE_NAME", cfg.App.AppName)
	}
}

func logLevelFromConfig(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func traceConfigFromAppConfig(cfg *config.AppConfig) infra.TraceConfig {
	if cfg == nil {
		return infra.TraceConfig{}
	}

	traceEndpoint := strings.TrimSpace(cfg.App.Trace.OTelResourceOTLPTraceEndpoint)
	usesTraceEndpoint := traceEndpoint != ""
	if !usesTraceEndpoint {
		traceEndpoint = strings.TrimSpace(cfg.App.Trace.OTLPExporterOTLPEndpoint)
	}

	protocol := strings.ToLower(strings.TrimSpace(cfg.App.Trace.OTLPExporterOTLPProtocol))
	if protocol == "" || protocol == "http" {
		protocol = "http/protobuf"
	}

	return infra.TraceConfig{
		Enabled:            cfg.App.Trace.Enabled,
		ServiceName:        cfg.App.AppName,
		OTLPEndpoint:       traceEndpoint,
		OTLPProtocol:       protocol,
		OTLPTracesEndpoint: usesTraceEndpoint,
		SamplerType:        "const",
		SamplerRate:        1,
	}
}

func initPprofFromConfig(cfg *config.AppConfig) (func(mux *http.ServeMux), func()) {
	addr := utils.StrOrDefault(
		strings.TrimSpace(os.Getenv("PPROF_LISTEN_ADDR")),
		infra.DEFAULT_PPROF_ADDR,
	)
	mode := strings.ToLower(strings.TrimSpace(cfg.App.Pprof.Mode))

	switch mode {
	case "server":
		if err := infra.InitGopprof(addr); err != nil {
			panic(err)
		}
		return nil, nil
	default:
		manager := infra.InitPprofServer(addr, "")
		return manager.RegisterHandlers, func() {
			if _, err := manager.Stop(); err != nil {
				log.GlobalLog().Error("failed to stop pprof", zap.Error(err))
			}
		}
	}
}

func initRedis(ctx context.Context, cfg *config.AppConfig) error {
	if cfg == nil {
		return nil
	}

	name, instance, ok, err := selectBootstrapInstance(cfg.Redis.Instances, "redis")
	if err != nil || !ok {
		return err
	}

	log.GlobalLog().Info("init redis from app config", zap.String("instance", name))
	return lunaredis.InitRedis(
		ctx,
		&redis.UniversalOptions{
			Addrs:    []string{fmt.Sprintf("%s:%d", instance.Addr, instance.Port)},
			Password: instance.Password,
			DB:       instance.DB,
		},
		defaultRedisTimeout,
		defaultRedisRetry,
	)
}

func initMySQL(cfg *config.AppConfig) error {
	if cfg == nil {
		return nil
	}

	name, instance, ok, err := selectBootstrapInstance(cfg.MySQL.Instances, "mysql")
	if err != nil || !ok {
		return err
	}

	driverConf := driverMySQL.Config{
		User:                 instance.Username,
		Passwd:               instance.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", instance.Host, instance.Port),
		DBName:               instance.Database,
		Params:               map[string]string{"charset": "utf8mb4"},
		ParseTime:            true,
		Loc:                  time.Local,
		AllowNativePasswords: true,
	}
	mysqlConf := gormMySQL.Config{
		DSN: driverConf.FormatDSN(),
	}

	log.GlobalLog().Info("init mysql from app config", zap.String("instance", name))
	if err := mysql.InitMySQL(
		mysqlConf,
		gorm.Config{
			Logger: mysql.NewMySQLGormLogger(mysqlConf),
		},
		func(db *gorm.DB) error {
			return nil
		},
	); err != nil {
		return err
	}

	db := mysql.GlobalMySQL()
	if db == nil {
		return fmt.Errorf("mysql init returned nil db")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	sqlDB.SetConnMaxIdleTime(cfg.MySQL.MaxIdleTime)
	if instance.PingEnabled {
		if err := sqlDB.Ping(); err != nil {
			return err
		}
	}

	return nil
}

func initKafka(cfg *config.AppConfig) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Kafka.Producer) == 0 && len(cfg.Kafka.Consumer) == 0 {
		return nil
	}

	log.GlobalLog().Warn(
		"kafka app config detected but bootstrap is not implemented in svc.Run yet",
		zap.Int("producer_count", len(cfg.Kafka.Producer)),
		zap.Int("consumer_count", len(cfg.Kafka.Consumer)),
	)
	return nil
}

func selectBootstrapInstance[T any](
	instances map[string]T,
	component string,
) (name string, instance T, ok bool, err error) {
	if len(instances) == 0 {
		return "", instance, false, nil
	}

	if inst, exists := instances["default"]; exists {
		return "default", inst, true, nil
	}

	if len(instances) == 1 {
		for instanceName, inst := range instances {
			return instanceName, inst, true, nil
		}
	}

	keys := make([]string, 0, len(instances))
	for key := range instances {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return "", instance, false, fmt.Errorf(
		"%s config defines multiple instances (%s) but svc.Run currently requires a \"default\" instance",
		component,
		strings.Join(keys, ", "),
	)
}
