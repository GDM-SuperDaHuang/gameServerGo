package log2

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log *zap.Logger

func Get() *zap.Logger {
	return log
}

type Config struct {
	Level       zapcore.Level
	LogDir      string // logs 根目录
	ServiceName string
	IsDocker    bool // 是否 docker 环境
}

func Init(cfg Config) {
	log = newLogger(cfg)
}

func newLogger(cfg Config) *zap.Logger {

	// ====== Docker 模式 ======
	if cfg.IsDocker {
		return newDockerLogger(cfg.Level)
	}

	// ====== 本地文件模式 ======
	today := time.Now().Format("2006-01-02")
	dayDir := filepath.Join(cfg.LogDir, today)

	_ = os.MkdirAll(dayDir, os.ModePerm)

	// 不同级别不同文件
	infoWriter := getFileWriter(filepath.Join(dayDir, "info.log"))
	errorWriter := getFileWriter(filepath.Join(dayDir, "error.log"))
	panicWriter := getFileWriter(filepath.Join(dayDir, "panic.log"))

	// encoder
	consoleEncoder := zapcore.NewConsoleEncoder(getConsoleEncoderConfig())
	jsonEncoder := zapcore.NewJSONEncoder(getJSONEncoderConfig())

	// core
	infoCore := zapcore.NewCore(
		jsonEncoder,
		infoWriter,
		zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l < zapcore.ErrorLevel
		}),
	)

	errorCore := zapcore.NewCore(
		jsonEncoder,
		errorWriter,
		zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l >= zapcore.ErrorLevel && l < zapcore.PanicLevel
		}),
	)

	panicCore := zapcore.NewCore(
		jsonEncoder,
		panicWriter,
		zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l >= zapcore.PanicLevel
		}),
	)

	// 控制台开发输出
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		cfg.Level,
	)

	core := zapcore.NewTee(infoCore, errorCore, panicCore, consoleCore)

	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

func newDockerLogger(level zapcore.Level) *zap.Logger {

	jsonEncoder := zapcore.NewJSONEncoder(getJSONEncoderConfig())

	core := zapcore.NewCore(
		jsonEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

func getConsoleEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder
	cfg.EncodeCaller = zapcore.ShortCallerEncoder
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg
}

func getJSONEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	cfg.EncodeCaller = zapcore.ShortCallerEncoder
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg
}

func getFileWriter(filename string) zapcore.WriteSyncer {
	hook := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100, // MB
		MaxBackups: 30,
		MaxAge:     30,
		Compress:   true,
	}
	return zapcore.AddSync(hook)
}
