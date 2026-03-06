package redis

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/tsumida/lunaship/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const redisLogMessage = "REDIS"

type redisTraceLogHook struct {
	instanceIP   string
	instancePort int
	tracer       oteltrace.Tracer
}

var _ redis.Hook = (*redisTraceLogHook)(nil)

func newRedisTraceLogHook(addrs []string) *redisTraceLogHook {
	instanceIP, instancePort := parseRedisAddrs(addrs)
	return &redisTraceLogHook{
		instanceIP:   instanceIP,
		instancePort: instancePort,
		tracer:       otel.Tracer("lunaship/redis"),
	}
}

func (h *redisTraceLogHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *redisTraceLogHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		begin := time.Now()
		spanName := "redis." + commandName(cmd)
		ctx, span := h.tracer.Start(
			ctx,
			spanName,
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithTimestamp(begin),
		)

		err := next(ctx, cmd)
		dur := time.Since(begin)
		h.endCommandSpan(span, cmd, dur, err)
		h.logCommand(ctx, cmd, dur, err)
		return err
	}
}

func (h *redisTraceLogHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		begin := time.Now()
		ctx, span := h.tracer.Start(
			ctx,
			"redis.pipeline",
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithTimestamp(begin),
			oteltrace.WithAttributes(
				attribute.String("target_type", "redis"),
				attribute.String("remote.ip", h.instanceIP),
				attribute.Int("remote.port", h.instancePort),
				attribute.Int("redis.command_count", len(cmds)),
			),
		)

		err := next(ctx, cmds)
		dur := time.Since(begin)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(
			attribute.Bool("error.flag", err != nil),
			attribute.Int64("duration.ms", dur.Milliseconds()),
		)
		span.End(oteltrace.WithTimestamp(begin.Add(dur)))

		fields := []zap.Field{
			zap.String("_instance_ip", h.instanceIP),
			zap.Int("_instance_port", h.instancePort),
			zap.Int("_redis_pipeline_cmd_count", len(cmds)),
			zap.Int64("_dur_ms", dur.Milliseconds()),
		}
		if err != nil {
			fields = append(fields, zap.NamedError("_error", err))
			log.Logger(ctx).Error(redisLogMessage, fields...)
			return err
		}
		log.Logger(ctx).Info(redisLogMessage, fields...)
		return nil
	}
}

func (h *redisTraceLogHook) endCommandSpan(span oteltrace.Span, cmd redis.Cmder, dur time.Duration, err error) {
	span.SetAttributes(
		attribute.String("target_type", "redis"),
		attribute.String("remote.ip", h.instanceIP),
		attribute.Int("remote.port", h.instancePort),
		attribute.String("redis.command", commandName(cmd)),
		attribute.Bool("error.flag", err != nil),
		attribute.Int64("duration.ms", dur.Milliseconds()),
	)
	if sha := luaSHAFromCommand(cmd); sha != "" {
		span.SetAttributes(attribute.String("lua.sha", sha))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func (h *redisTraceLogHook) logCommand(ctx context.Context, cmd redis.Cmder, dur time.Duration, err error) {
	fields := []zap.Field{
		zap.String("_instance_ip", h.instanceIP),
		zap.Int("_instance_port", h.instancePort),
		zap.String("_redis_cmd", commandName(cmd)),
		zap.Int64("_dur_ms", dur.Milliseconds()),
	}
	if sha := luaSHAFromCommand(cmd); sha != "" {
		fields = append(fields, zap.String("_redis_lua_sha", sha))
	}
	if err != nil {
		fields = append(fields, zap.NamedError("_error", err))
		log.Logger(ctx).Error(redisLogMessage, fields...)
		return
	}
	log.Logger(ctx).Info(redisLogMessage, fields...)
}

func commandName(cmd redis.Cmder) string {
	if cmd == nil {
		return "unknown"
	}
	name := strings.TrimSpace(strings.ToLower(cmd.Name()))
	if name != "" {
		return name
	}
	return "unknown"
}

func luaSHAFromCommand(cmd redis.Cmder) string {
	if cmd == nil {
		return ""
	}
	if strings.ToLower(strings.TrimSpace(cmd.Name())) != "evalsha" {
		return ""
	}
	args := cmd.Args()
	if len(args) < 2 {
		return ""
	}
	sha, _ := args[1].(string)
	return strings.TrimSpace(sha)
}

func parseRedisAddrs(addrs []string) (string, int) {
	for _, addr := range addrs {
		host, port := parseRedisAddr(addr)
		if host != "" || port > 0 {
			return host, port
		}
	}
	return "", 0
}

func parseRedisAddr(raw string) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}
	if host, port, err := net.SplitHostPort(raw); err == nil {
		return strings.TrimSpace(host), parseRedisPort(port)
	}
	lastColon := strings.LastIndex(raw, ":")
	if lastColon > 0 && lastColon < len(raw)-1 {
		return strings.TrimSpace(raw[:lastColon]), parseRedisPort(raw[lastColon+1:])
	}
	return raw, 0
}

func parseRedisPort(raw string) int {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port < 0 {
		return 0
	}
	return port
}
