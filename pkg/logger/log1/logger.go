package log1

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

// Get ..
func Get() *zap.Logger {
	return logger
}

// Init 初始化全局日志
func Init(level zapcore.Level, name, filePath string, isJSONStyle bool) {
	logger = New(level, name, filePath, isJSONStyle)
}

// New 创建日志
func New(level zapcore.Level, name, filePath string, isJSONStyle bool) *zap.Logger {
	encoderConfig := getEncoderConfig()
	ioWrite := createWriteSyncers(level, name, filePath)
	options := createOptions(level)

	var encoder zapcore.Encoder
	if isJSONStyle {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,                                 // 编码器配置
		zapcore.NewMultiWriteSyncer(ioWrite...), // 打印到那里
		zap.NewAtomicLevelAt(level),             // 日志级别
	)

	return zap.New(core, options...)
}

// todo
func getEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "linenum",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseColorLevelEncoder, // 小写彩色编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,         // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,     // 时间格式化
		EncodeCaller:   zapcore.FullCallerEncoder,          // 长路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}
}

func createWriteSyncers(level zapcore.Level, name, filePath string) []zapcore.WriteSyncer {
	ioWrite := make([]zapcore.WriteSyncer, 0)

	// 打印到控制台
	if level.Enabled(zapcore.DebugLevel) || len(filePath) == 0 {
		ioWrite = append(ioWrite, zapcore.AddSync(os.Stdout))
	}

	// 打印到文件
	if len(filePath) > 0 {
		fullFilePath := buildLogFilePath(name, filePath)
		hook := lumberjack.Logger{
			Filename:   fullFilePath, // 日志文件路径
			MaxSize:    128,          // 每个日志文件保存的最大尺寸 单位: M
			MaxBackups: 30,           // 日志文件最多保存多少个备份
			MaxAge:     30,           // 文件最多保存多少天
			Compress:   false,        // 是否压缩
		}
		ioWrite = append(ioWrite, zapcore.AddSync(&hook))
	}

	return ioWrite
}

func buildLogFilePath(name, filePath string) string {
	if filePath[len(filePath)-1] == '/' {
		return filePath + name + ".log"
	}
	return filePath + "/" + name + ".log"
}

func createOptions(level zapcore.Level) []zap.Option {
	options := []zap.Option{
		zap.AddCaller(),                       // 开启文件及行号
		zap.AddStacktrace(zapcore.ErrorLevel), // 打印stack
	}

	if level.Enabled(zapcore.DebugLevel) {
		options = append(options, zap.Development())
	}

	return options
}
