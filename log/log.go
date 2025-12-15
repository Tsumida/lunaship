package log

import (
	"io"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logging format for infra.
// Ref: https://zhuanlan.zhihu.com/p/141321801

var (
	logger *zap.Logger
)

//go:inline
func GlobalLog() *zap.Logger {
	return logger
}

func getWriter(filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   false,
	}
}

// 初始化日志 logger
func InitLog(logPath, errPath string, logLevel zapcore.Level, isDev bool) *zap.Logger {
	config := zapcore.EncoderConfig{
		MessageKey:   "msg",                       //结构化（json）输出：msg的key
		LevelKey:     "level",                     //结构化（json）输出：日志级别的key（INFO，WARN，ERROR等）
		TimeKey:      "ts",                        //结构化（json）输出：时间的key（INFO，WARN，ERROR等）
		CallerKey:    "file",                      //结构化（json）输出：打印日志的文件对应的Key
		EncodeLevel:  zapcore.CapitalLevelEncoder, //将日志级别转换成大写（INFO，WARN，ERROR等）
		EncodeCaller: zapcore.ShortCallerEncoder,  //采用短文件路径编码输出（test/main.go:14 ）
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format(time.RFC3339))
		}, //输出的时间格式
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		}, //
	}
	//自定义日志级别：自定义Info级别
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.WarnLevel && lvl >= logLevel
	})
	//自定义日志级别：自定义Warn级别
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel && lvl >= logLevel
	})

	infoWriter := getWriter(logPath)
	warnWriter := getWriter(errPath)

	// multiple output

	outputList := []zapcore.Core{
		zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.AddSync(infoWriter), infoLevel), //将info及以下写入logPath，NewConsoleEncoder 是非结构化输出
		zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.AddSync(warnWriter), warnLevel), //warn及以上写入errPath
	}

	if isDev {
		outputList = append(
			outputList,
			//同时将日志输出到控制台，NewJSONEncoder 是结构化输出
			zapcore.NewCore(
				zapcore.NewJSONEncoder(config),
				zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
				logLevel),
		)
	}

	core := zapcore.NewTee(outputList...)
	logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	return logger
}
