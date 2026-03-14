package mysql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	driverMySQL "github.com/go-sql-driver/mysql"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	gormMySQL "gorm.io/driver/mysql"
	gormLogger "gorm.io/gorm/logger"
)

const defaultMySQLSlowThreshold = 200 * time.Millisecond
const sqlLogMessage = "SQL"

type mysqlGormLogger struct {
	logLevel      gormLogger.LogLevel
	slowThreshold time.Duration
	instanceIP    string
	instancePort  int
	database      string
	loggerFn      func(context.Context) *zap.Logger
}

// Compile-time assertion: mysqlGormLogger must keep implementing gorm logger.Interface.
var _ gormLogger.Interface = (*mysqlGormLogger)(nil)

func NewMySQLGormLogger(conf gormMySQL.Config) gormLogger.Interface {
	instanceIP, instancePort, database := mysqlEndpoint(conf.DSN)
	return &mysqlGormLogger{
		logLevel:      gormLogger.Info,
		slowThreshold: defaultMySQLSlowThreshold,
		instanceIP:    instanceIP,
		instancePort:  instancePort,
		database:      database,
		loggerFn:      log.Logger,
	}
}

func (l *mysqlGormLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	cloned := *l
	cloned.logLevel = level
	return &cloned
}

func (l *mysqlGormLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.logLevel < gormLogger.Info {
		return
	}
	l.logger(ctx).Info(fmt.Sprintf(msg, args...))
}

func (l *mysqlGormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.logLevel < gormLogger.Warn {
		return
	}
	l.logger(ctx).Warn(fmt.Sprintf(msg, args...))
}

func (l *mysqlGormLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.logLevel < gormLogger.Error {
		return
	}
	l.logger(ctx).Error(fmt.Sprintf(msg, args...))
}

func (l *mysqlGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sqlText, _ := fc()
	dur := time.Since(begin)
	isSlow := l.slowThreshold > 0 && dur > l.slowThreshold
	emitErr := err != nil && !errors.Is(err, gormLogger.ErrRecordNotFound)
	l.emitTraceSpan(ctx, begin, dur, emitErr, err)

	fields := l.sqlFields(sqlText, dur, isSlow)
	logger := l.logger(ctx)

	if emitErr {
		if l.logLevel < gormLogger.Error {
			return
		}
		fields = append(fields, zap.NamedError("_error", err))
		logger.Error(sqlLogMessage, fields...)
		return
	}
	if isSlow {
		if l.logLevel < gormLogger.Warn {
			return
		}
		logger.Warn(sqlLogMessage, fields...)
		return
	}
	if l.logLevel < gormLogger.Info {
		return
	}
	logger.Info(sqlLogMessage, fields...)
}

func (l *mysqlGormLogger) emitTraceSpan(
	ctx context.Context,
	begin time.Time,
	dur time.Duration,
	hasErr bool,
	err error,
) {
	tracer := otel.Tracer("MySQL")
	_, span := tracer.Start(
		ctx,
		"mysql",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithTimestamp(begin),
		oteltrace.WithAttributes(
			attribute.String("target_type", "mysql"),
			attribute.String("remote.ip", l.instanceIP),
			attribute.Int("remote.port", l.instancePort),
			attribute.Bool("error.flag", hasErr),
			attribute.Int64("duration.ms", dur.Milliseconds()),
		),
	)
	if hasErr {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End(oteltrace.WithTimestamp(begin.Add(dur)))
}

func (l *mysqlGormLogger) sqlFields(sqlText string, dur time.Duration, isSlow bool) []zap.Field {
	return []zap.Field{
		zap.String("_instance_ip", l.instanceIP),
		zap.Int("_instance_port", l.instancePort),
		zap.String("_database", l.database),
		zap.Int64("_dur_ms", dur.Milliseconds()),
		zap.String("_sql", sqlText),
		zap.Bool("_is_slow_query", isSlow),
	}
}

func (l *mysqlGormLogger) logger(ctx context.Context) *zap.Logger {
	if l.loggerFn != nil {
		if logger := l.loggerFn(ctx); logger != nil {
			return logger
		}
	}
	if logger := log.GlobalLog(); logger != nil {
		return logger
	}
	return zap.NewNop()
}

func mysqlEndpoint(dsn string) (string, int, string) {
	cfg, err := driverMySQL.ParseDSN(dsn)
	if err != nil {
		return "", 0, ""
	}
	host, port := parseHostPort(cfg.Addr)
	return host, port, cfg.DBName
}

func parseHostPort(addr string) (string, int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err == nil {
		return host, parsePort(portStr)
	}
	lastColon := strings.LastIndex(addr, ":")
	if lastColon > 0 && lastColon < len(addr)-1 {
		return strings.TrimSpace(addr[:lastColon]), parsePort(addr[lastColon+1:])
	}
	return addr, 0
}

func parsePort(raw string) int {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port < 0 {
		return 0
	}
	return port
}
