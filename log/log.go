package log

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

var MLog = &Log{}

type Log struct {
	logger *zerolog.Logger
}

// InitLog 初始化日志系统
func (l *Log) InitLog(logConfig map[string]string, logFileName string) *zerolog.Logger {
	logType := strings.ToLower(logConfig["type"]) // console / file / hybrid
	useColor := strings.EqualFold(logConfig["color"], "true")

	// 默认日志等级
	levelStr := strings.ToLower(logConfig["level"])
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// ========== 日志输出 Writer 组合 ==========
	var writers []io.Writer

	// 控制台输出
	if logType == "console" || logType == "hybrid" {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
			NoColor:    !useColor,
		}
		writers = append(writers, consoleWriter)
	}

	// 文件输出
	if logType == "file" || logType == "hybrid" {
		infoWriter := getWriter(fmt.Sprintf("./logs/%s_info.log", logFileName), logConfig)
		errorWriter := getWriter(fmt.Sprintf("./logs/%s_error.log", logFileName), logConfig)
		writers = append(writers, multiLevelWriter(infoWriter, errorWriter))
	}

	multi := io.MultiWriter(writers...)
	logger := zerolog.New(multi).With().Timestamp().Caller().Logger()

	l.logger = &logger
	return l.logger
}

// GetLog 获取 zerolog.Logger
func (l *Log) GetLog() *zerolog.Logger {
	return l.logger
}

// getWriter 日志文件滚动
func getWriter(filename string, logConfig map[string]string) io.Writer {
	// 读取配置，提供默认值
	maxSize := getIntConfig(logConfig, "maxSize", 10)
	maxBackups := getIntConfig(logConfig, "maxBackups", 7)
	maxAge := getIntConfig(logConfig, "maxAge", 30)
	compress := getBoolConfig(logConfig, "compress", true)

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}
}

// getIntConfig 从配置中读取整数值，提供默认值
func getIntConfig(config map[string]string, key string, defaultValue int) int {
	if val, ok := config[key]; ok {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getBoolConfig 从配置中读取布尔值，提供默认值
func getBoolConfig(config map[string]string, key string, defaultValue bool) bool {
	if val, ok := config[key]; ok {
		return strings.EqualFold(val, "true")
	}
	return defaultValue
}

// multiLevelWriter 区分 info / error 级别文件
func multiLevelWriter(infoWriter, errorWriter io.Writer) io.Writer {
	return zerolog.MultiLevelWriter(levelWriter{
		infoWriter:  infoWriter,
		errorWriter: errorWriter,
	})
}

// levelWriter 实现根据日志级别分流到不同文件
type levelWriter struct {
	infoWriter  io.Writer
	errorWriter io.Writer
}

func (lw levelWriter) Write(p []byte) (n int, err error) {
	// 默认写入 info 文件
	return lw.infoWriter.Write(p)
}

func (lw levelWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	switch level {
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		return lw.errorWriter.Write(p)
	default:
		return lw.infoWriter.Write(p)
	}
}
