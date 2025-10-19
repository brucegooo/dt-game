package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var log *zap.Logger
var atomicLevel zap.AtomicLevel

// InitLogger 初始化全局日志器，支持通过环境变量控制：
// - LOG_LEVEL=debug|info|warn|error（默认：info）
// - LOG_TO_FILE=true|false（默认：false）或提供 LOG_FILE/LOG_DIR 之一则启用文件输出
// - LOG_FILE=./logs/app.log（优先级高于 LOG_DIR）
// - LOG_DIR=./logs（若设置则默认写入 logs/app.log）
// - LOG_MAX_SIZE_MB=100、LOG_MAX_BACKUPS=7、LOG_MAX_DAYS=14、LOG_COMPRESS=true
func InitLogger() {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.LevelKey = "level"
	encoderConfig.NameKey = "logger"
	encoderConfig.CallerKey = "caller"
	encoderConfig.MessageKey = "msg"
	encoderConfig.StacktraceKey = "stacktrace"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	// 日志级别
	levelStr := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	zapLevel := zapcore.InfoLevel
	switch levelStr {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info", "":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	}
	atomicLevel = zap.NewAtomicLevelAt(zapLevel)

	enc := zapcore.NewJSONEncoder(encoderConfig)
	cores := []zapcore.Core{
		zapcore.NewCore(enc, zapcore.Lock(os.Stdout), atomicLevel),
	}

	// 文件日志（可选）
	logToFile := strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_TO_FILE")), "true")
	logFile := strings.TrimSpace(os.Getenv("LOG_FILE"))
	logDir := strings.TrimSpace(os.Getenv("LOG_DIR"))
	if logFile == "" && logDir != "" {
		logFile = filepath.Join(logDir, "app.log")
	}
	if logToFile || logFile != "" {
		if logFile == "" {
			logFile = filepath.Join(".", "logs", "app.log")
		}
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			// 如果无法创建日志目录，仅输出到 stdout，不中断程序
			_, _ = fmt.Fprintf(os.Stderr, "warning: failed to create log directory %s: %v\n", logDir, err)
			return
		}

		maxSize := getenvInt("LOG_MAX_SIZE_MB", 100)
		maxBackups := getenvInt("LOG_MAX_BACKUPS", 7)
		maxAge := getenvInt("LOG_MAX_DAYS", 14)
		compress := getenvBool("LOG_COMPRESS", true)

		lw := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
		}
		cores = append(cores, zapcore.NewCore(enc, zapcore.AddSync(lw), atomicLevel))
	}

	log = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1))
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	}
	return def
}

func Info(msg string, fields ...zap.Field)   { log.Info(msg, fields...) }
func Error(msg string, fields ...zap.Field)  { log.Error(msg, fields...) }
func Warn(msg string, fields ...zap.Field)   { log.Warn(msg, fields...) }
func Debug(msg string, fields ...zap.Field)  { log.Debug(msg, fields...) }
func Fatalf(msg string, fields ...zap.Field) { log.Fatal(msg, fields...) }
func Sync()                                  { _ = log.Sync() }

// SetLevel 动态调整日志级别（debug/info/warn/error）
func SetLevel(level string) {
	ls := strings.ToLower(strings.TrimSpace(level))
	switch ls {
	case "debug":
		atomicLevel.SetLevel(zapcore.DebugLevel)
	case "info":
		atomicLevel.SetLevel(zapcore.InfoLevel)
	case "warn", "warning":
		atomicLevel.SetLevel(zapcore.WarnLevel)
	case "error":
		atomicLevel.SetLevel(zapcore.ErrorLevel)
	default:
		// 无效级别忽略
	}
}

// 封装结构体字段统一处理
func fieldsWithTrace(ctx context.Context, fields ...zap.Field) []zap.Field {
	traceId := GetTraceID(ctx)
	if traceId != "" {
		fields = append(fields, zap.String("traceId", traceId))
	}
	return fields
}

func InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	log.Info(msg, fieldsWithTrace(ctx, fields...)...)
}
func ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	log.Error(msg, fieldsWithTrace(ctx, fields...)...)
}
func WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	log.Warn(msg, fieldsWithTrace(ctx, fields...)...)
}
func DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	log.Debug(msg, fieldsWithTrace(ctx, fields...)...)
}
