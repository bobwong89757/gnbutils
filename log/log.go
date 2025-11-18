package log

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var MLog = &Log{}

type Log struct {
	logger *zap.SugaredLogger
}

func (l *Log) InitLog(logConfig map[string]string, logFileName string) {
	// 解析日志级别（默认 debug）
	minLevel := zapcore.DebugLevel
	if levelStr, ok := logConfig["level"]; ok && levelStr != "" {
		levelStr = strings.ToLower(strings.TrimSpace(levelStr))
		switch levelStr {
		case "debug":
			minLevel = zapcore.DebugLevel
		case "info":
			minLevel = zapcore.InfoLevel
		case "warn", "warning":
			minLevel = zapcore.WarnLevel
		case "error":
			minLevel = zapcore.ErrorLevel
		case "fatal":
			minLevel = zapcore.FatalLevel
		case "panic":
			minLevel = zapcore.PanicLevel
		}
	}

	levelEncoder := zapcore.CapitalLevelEncoder
	useColor, ok := logConfig["color"]
	if ok {
		if strings.EqualFold(useColor, "true") {
			levelEncoder = zapcore.CapitalColorLevelEncoder
		}
	}

	logType := logConfig["type"]

	// 设置一些基本日志格式 具体含义还比较好理解，直接看zap源码也不难懂
	encoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: levelEncoder,
		TimeKey:     "ts",
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		},
		CallerKey:    "file",
		EncodeCaller: zapcore.ShortCallerEncoder,
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		},
	})

	// 创建各级别的 LevelEnabler（根据配置的最小级别过滤）
	debugLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel && lvl >= zapcore.DebugLevel && lvl < zapcore.InfoLevel
	})

	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel && lvl >= zapcore.InfoLevel && lvl < zapcore.WarnLevel
	})

	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel && lvl >= zapcore.WarnLevel && lvl < zapcore.ErrorLevel
	})

	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel && lvl >= zapcore.ErrorLevel
	})

	// 获取各级别日志文件的io.Writer
	debugWriter := getWriter(fmt.Sprintf("./logs/%s_debug.log", logFileName), logConfig)
	infoWriter := getWriter(fmt.Sprintf("./logs/%s_info.log", logFileName), logConfig)
	warnWriter := getWriter(fmt.Sprintf("./logs/%s_warn.log", logFileName), logConfig)
	errorWriter := getWriter(fmt.Sprintf("./logs/%s_error.log", logFileName), logConfig)

	// 创建全局级别过滤器（用于控制整体日志输出）
	globalLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel
	})

	// 根据 logType 创建不同的 core
	var cores []zapcore.Core

	if strings.EqualFold(logType, "console") {
		// 控制台输出：所有级别都输出到控制台
		cores = append(cores,
			zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), globalLevel),
		)
	} else if strings.EqualFold(logType, "file") {
		// 文件输出：分别输出到不同级别的文件
		if minLevel <= zapcore.DebugLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(debugWriter), debugLevel))
		}
		if minLevel <= zapcore.InfoLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel))
		}
		if minLevel <= zapcore.WarnLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), warnLevel))
		}
		if minLevel <= zapcore.ErrorLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel))
		}
	} else if strings.EqualFold(logType, "hybrid") {
		// 混合输出：控制台 + 文件
		// 控制台输出所有级别
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), globalLevel))
		// 文件分别输出到不同级别
		if minLevel <= zapcore.DebugLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(debugWriter), debugLevel))
		}
		if minLevel <= zapcore.InfoLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel))
		}
		if minLevel <= zapcore.WarnLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), warnLevel))
		}
		if minLevel <= zapcore.ErrorLevel {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel))
		}
	} else {
		// 默认：控制台输出
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), globalLevel))
	}

	// 创建 Tee core
	core := zapcore.NewTee(cores...)

	// 需要传入 zap.AddCaller() 才会显示打日志点的文件名和行数
	log := zap.New(core, zap.AddCaller())
	l.logger = log.Sugar()
}

func (l *Log) GetLog() *zap.SugaredLogger {
	return l.logger
}

// getWriter 创建日志文件 Writer，支持通过配置设置切割参数
// filename: 日志文件路径
// logConfig: 日志配置 map，支持以下配置项：
//   - maxAge: 保留天数（默认7天）。如果设置为 -1，则禁用基于时间的清理
//   - rotationTime: 分割时间间隔，支持格式：1h, 30m, 24h, 1d（默认1d，即24小时）
//   - rotationCount: 保留的文件数量（默认-1，表示不限制）。如果设置了此选项，需要将 maxAge 设置为 -1
//   - rotationFormat: 文件名格式，如 "%Y%m%d"（默认根据 rotationTime 自动选择）
//
// 注意：file-rotatelogs 不支持按大小分割，只支持按时间分割
// 如果需要按大小分割，请考虑使用其他日志库（如 lumberjack）
func getWriter(filename string, logConfig map[string]string) io.Writer {
	// 解析保留天数（默认7天）
	maxAgeDays := 7
	if maxAgeStr, ok := logConfig["maxAge"]; ok && maxAgeStr != "" {
		if days, err := strconv.Atoi(maxAgeStr); err == nil {
			maxAgeDays = days
		}
	}

	// 解析分割时间间隔（默认1天）
	rotationTime := 24 * time.Hour
	if rotationTimeStr, ok := logConfig["rotationTime"]; ok && rotationTimeStr != "" {
		if duration, err := parseDuration(rotationTimeStr); err == nil && duration > 0 {
			rotationTime = duration
		}
	}

	// 解析文件名格式（默认根据 rotationTime 自动选择）
	rotationFormat := ""
	if format, ok := logConfig["rotationFormat"]; ok && format != "" {
		rotationFormat = format
	} else {
		// 根据 rotationTime 自动选择合适的格式
		if rotationTime >= 24*time.Hour {
			rotationFormat = "%Y%m%d" // 按天
		} else if rotationTime >= time.Hour {
			rotationFormat = "%Y%m%d%H" // 按小时
		} else {
			rotationFormat = "%Y%m%d%H%M" // 按分钟
		}
	}

	// 构建文件名模式
	baseName := strings.Replace(filename, ".log", "", -1)
	pattern := fmt.Sprintf("%s-%s.log", baseName, rotationFormat)

	// 解析保留文件数量
	rotationCount := -1
	if rotationCountStr, ok := logConfig["rotationCount"]; ok && rotationCountStr != "" {
		if count, err := strconv.Atoi(rotationCountStr); err == nil && count > 0 {
			rotationCount = count
		}
	}

	// 创建 rotatelogs 选项
	options := []rotatelogs.Option{
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithRotationTime(rotationTime),
	}

	// 设置 MaxAge 或 RotationCount
	// 如果设置了 rotationCount，必须将 maxAge 设置为 -1
	if rotationCount > 0 {
		// 使用 rotationCount 模式：保留固定数量的文件
		options = append(options, rotatelogs.WithMaxAge(-1))
		options = append(options, rotatelogs.WithRotationCount(uint(rotationCount)))
	} else {
		// 使用 maxAge 模式：基于时间清理
		if maxAgeDays >= 0 {
			options = append(options, rotatelogs.WithMaxAge(time.Duration(maxAgeDays)*24*time.Hour))
		} else {
			// maxAge = -1 表示禁用基于时间的清理
			options = append(options, rotatelogs.WithMaxAge(-1))
		}
	}

	// 创建 rotatelogs Logger
	hook, err := rotatelogs.New(pattern, options...)
	if err != nil {
		panic(fmt.Errorf("failed to create log writer: %w", err))
	}

	return hook
}

// parseDuration 解析时间字符串，支持格式：1h, 30m, 24h, 1d, 7d 等
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	// 支持 "d" 表示天
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// 使用标准库解析其他格式（h, m, s）
	return time.ParseDuration(s)
}
