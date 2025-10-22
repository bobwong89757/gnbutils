package log

import (
	"fmt"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"io"
	"os"
	"strings"
)

var MLog = &Log{}

type Log struct {
	logger zerolog.Logger
}

// InitLog 初始化日志系统
func (l *Log) InitLog(logConfig map[string]string, logFileName string) {
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
		infoWriter := getWriter(fmt.Sprintf("./logs/%s_info.log", logFileName))
		errorWriter := getWriter(fmt.Sprintf("./logs/%s_error.log", logFileName))
		writers = append(writers, multiLevelWriter(infoWriter, errorWriter))
	}

	multi := io.MultiWriter(writers...)
	logger := zerolog.New(multi).With().Timestamp().Caller().Logger()

	l.logger = logger
}

// GetLog 获取 zerolog.Logger
func (l *Log) GetLog() zerolog.Logger {
	return l.logger
}

// getWriter 日志文件滚动
func getWriter(filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,  // 单文件最大100MB
		MaxBackups: 7,    // 保留7个旧文件
		MaxAge:     30,   // 保留30天
		Compress:   true, // 压缩旧日志
	}
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
