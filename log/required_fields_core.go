package log

import (
	"net"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const maxLogTextLen = 1024

var fieldKeyAliases = map[string]string{
	"trace_id":       "_trace_id",
	"span_id":        "_span_id",
	"parent_span_id": "_parent_span_id",
	"server_ip":      "_app_ip",
	"server_port":    "_app_port",
	"duration_ms":    "_dur_ms",
	"sql":            "_sql",
}

var (
	appName string
	appIP   string
	appPort int
)

// AppIdentity returns values injected as common app identity fields.
func AppIdentity() (name, ip string, port int) {
	return appName, appIP, appPort
}

type requiredFieldsCore struct {
	zapcore.Core
	defaults    []zap.Field
	boundFields map[string]struct{}
}

func newRequiredFieldsCore(core zapcore.Core, defaults []zap.Field) zapcore.Core {
	return &requiredFieldsCore{
		Core:        core,
		defaults:    defaults,
		boundFields: map[string]struct{}{},
	}
}

func (c *requiredFieldsCore) With(fields []zap.Field) zapcore.Core {
	normalized := normalizeFields(fields)
	bound := make(map[string]struct{}, len(c.boundFields)+len(normalized))
	for key := range c.boundFields {
		bound[key] = struct{}{}
	}
	for _, field := range normalized {
		bound[field.Key] = struct{}{}
	}
	return &requiredFieldsCore{
		Core:        c.Core.With(normalized),
		defaults:    c.defaults,
		boundFields: bound,
	}
}

func (c *requiredFieldsCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *requiredFieldsCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	ent.Message = truncateUTF8(ent.Message, maxLogTextLen)
	normalized := normalizeFields(fields)

	present := make(map[string]struct{}, len(c.boundFields)+len(normalized))
	for key := range c.boundFields {
		present[key] = struct{}{}
	}

	merged := make([]zap.Field, 0, len(normalized)+len(c.defaults))
	merged = append(merged, normalized...)
	for _, field := range normalized {
		present[field.Key] = struct{}{}
	}
	for _, field := range c.defaults {
		if _, ok := present[field.Key]; ok {
			continue
		}
		merged = append(merged, field)
	}

	return c.Core.Write(ent, merged)
}

func normalizeFields(fields []zap.Field) []zap.Field {
	normalized := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		if alias, ok := fieldKeyAliases[field.Key]; ok {
			field.Key = alias
		}
		if field.Key == "_sql" && field.Type == zapcore.StringType {
			field.String = truncateUTF8(field.String, maxLogTextLen)
		}
		normalized = append(normalized, field)
	}
	return normalized
}

func defaultInjectedFieldsFromEnv() []zap.Field {
	env := firstNonEmpty(
		strings.TrimSpace(os.Getenv("APP_ENV")),
		strings.TrimSpace(os.Getenv("DEPLOY_ENV")),
		strings.TrimSpace(os.Getenv("ENV")),
	)
	app := firstNonEmpty(
		strings.TrimSpace(os.Getenv("APP_NAME")),
		strings.TrimSpace(os.Getenv("SERVICE_ID")),
		strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")),
	)

	ip := firstNonEmpty(
		strings.TrimSpace(os.Getenv("APP_IP")),
		strings.TrimSpace(os.Getenv("POD_IP")),
	)
	port := parsePositiveInt(strings.TrimSpace(os.Getenv("APP_PORT")))

	bindHost, bindPort := parseBindAddress(strings.TrimSpace(os.Getenv("BIND_ADDR")))
	if ip == "" {
		ip = bindHost
	}
	if port == 0 {
		port = bindPort
	}

	appName = app
	appIP = ip
	appPort = port

	return []zap.Field{
		zap.String("_env", env),
		zap.String("_app", app),
		zap.String("_app_ip", ip),
		zap.Int("_app_port", port),
		zap.String("_trace_id", ""),
		zap.String("_span_id", ""),
	}
}

func parseBindAddress(bindAddr string) (string, int) {
	if bindAddr == "" {
		return "", 0
	}
	if strings.HasPrefix(bindAddr, ":") {
		return "", parsePositiveInt(strings.TrimPrefix(bindAddr, ":"))
	}
	if host, port, err := net.SplitHostPort(bindAddr); err == nil {
		return strings.TrimSpace(host), parsePositiveInt(strings.TrimSpace(port))
	}
	return "", parsePositiveInt(bindAddr)
}

func parsePositiveInt(raw string) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func truncateUTF8(s string, maxChars int) string {
	if maxChars <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	count := 0
	for idx := range s {
		if count == maxChars {
			return s[:idx]
		}
		count++
	}
	return s
}
