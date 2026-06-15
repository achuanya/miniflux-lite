// Package logger 根据配置初始化一个 Uber Zap 日志实例。
//
// 支持三项动态配置：
//   - 日志级别（LOG_LEVEL）：控制输出哪些级别的日志；
//   - 输出目标（LOG_FILE_PATH）：为空时输出到 stdout，否则写入指定文件；
//   - 编码格式（LOG_ENCODER）：json（结构化）或 console（易读）。
package logger

import (
	"fmt"
	"os"

	"miniflux-lite/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 依据 cfg 构造并返回一个 *zap.Logger。
//
// 调用方应在使用结束后执行 `defer logger.Sync()` 以刷新缓冲。
func New(cfg *config.Config) (*zap.Logger, error) {
	// 解析日志级别。ParseAtomicLevel 支持 debug/info/warn/error/dpanic/panic/fatal，
	// 且大小写不敏感。
	level, err := zap.ParseAtomicLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("无效的日志级别 LOG_LEVEL=%q: %w", cfg.LogLevel, err)
	}

	// 选择编码器。
	encoder, err := buildEncoder(cfg.LogEncoder)
	if err != nil {
		return nil, err
	}

	// 选择输出目标：文件或标准输出。
	writeSyncer, err := buildWriteSyncer(cfg.LogFilePath)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	// AddCaller 在日志中附加调用位置，便于定位问题。
	return zap.New(core, zap.AddCaller()), nil
}

// buildEncoder 根据格式名构造编码器。
func buildEncoder(encoderName string) (zapcore.Encoder, error) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder, // 大写级别，如 INFO
		EncodeTime:     zapcore.ISO8601TimeEncoder,  // ISO8601 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	switch encoderName {
	case "json":
		return zapcore.NewJSONEncoder(encoderCfg), nil
	case "console":
		// 控制台模式下为级别添加颜色，提升可读性。
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return zapcore.NewConsoleEncoder(encoderCfg), nil
	default:
		// 理论上 config 校验已拦截，这里作为防御性兜底。
		return nil, fmt.Errorf("不支持的日志格式 LOG_ENCODER=%q", encoderName)
	}
}

// buildWriteSyncer 根据 filePath 选择输出目标。
// filePath 为空时输出到 stdout；否则以追加方式打开/创建文件。
func buildWriteSyncer(filePath string) (zapcore.WriteSyncer, error) {
	if filePath == "" {
		return zapcore.AddSync(os.Stdout), nil
	}

	// O_APPEND：追加写入，避免覆盖既有日志；O_CREATE：不存在则创建。
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败 LOG_FILE_PATH=%q: %w", filePath, err)
	}
	return zapcore.AddSync(file), nil
}
