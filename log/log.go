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

	// 检查是否启用异步写入
	useAsync := false
	if asyncStr, ok := logConfig["async"]; ok {
		useAsync = strings.EqualFold(asyncStr, "true") || asyncStr == "1"
	}

	// 解析文件输出模式：separate（分别输出到不同级别文件）或 single（统一输出到一个文件）
	fileMode := "separate"
	if mode, ok := logConfig["fileMode"]; ok {
		fileMode = strings.ToLower(strings.TrimSpace(mode))
	}

	// 解析控制台输出级别（可选，格式：debug,info,warn,error 或 all）
	consoleLevels := parseOutputLevels(logConfig["consoleLevels"], minLevel)
	// 解析文件输出级别（可选，格式：debug,info,warn,error 或 all）
	fileLevels := parseOutputLevels(logConfig["fileLevels"], minLevel)

	// 创建全局级别过滤器（用于控制整体日志输出）
	globalLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= minLevel
	})

	// 辅助函数：根据配置决定是否使用异步写入包装 writer
	wrapWriter := func(w io.Writer) zapcore.WriteSyncer {
		ws := zapcore.AddSync(w)
		if useAsync {
			return newAsyncWriter(ws)
		}
		return ws
	}

	// 根据 logType 创建不同的 core
	var cores []zapcore.Core

	// 判断是否需要控制台输出
	needConsole := strings.EqualFold(logType, "console") || strings.EqualFold(logType, "hybrid") || logType == ""
	if needConsole {
		consoleLevel := globalLevel
		if len(consoleLevels) > 0 {
			// 使用配置的控制台级别
			consoleLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= minLevel && containsLevel(consoleLevels, lvl)
			})
		}
		cores = append(cores, zapcore.NewCore(encoder, wrapWriter(os.Stdout), consoleLevel))
	}

	// 判断是否需要文件输出
	needFile := strings.EqualFold(logType, "file") || strings.EqualFold(logType, "hybrid")
	if needFile {
		if fileMode == "single" {
			// 统一文件输出：所有级别输出到一个文件
			fileLevel := globalLevel
			if len(fileLevels) > 0 {
				fileLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
					return lvl >= minLevel && containsLevel(fileLevels, lvl)
				})
			}
			allWriter := getWriter(fmt.Sprintf("./logs/%s.log", logFileName), logConfig)
			cores = append(cores, zapcore.NewCore(encoder, wrapWriter(allWriter), fileLevel))
		} else {
			// 分别输出到不同级别的文件（使用精确匹配，避免创建不需要的文件）
			cores = append(cores, buildFileCores(encoder, wrapWriter, logFileName, logConfig, minLevel, fileLevels)...)
		}
	}

	// 检查是否有有效的 cores
	if len(cores) == 0 {
		// 如果没有有效的 cores，至少创建一个控制台输出，避免完全没有日志输出
		cores = append(cores, zapcore.NewCore(encoder, wrapWriter(os.Stdout), globalLevel))
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

// parseOutputLevels 解析输出级别配置，支持格式：debug,info,warn,error 或 all
// 返回需要输出的级别列表，如果为空表示输出所有级别
func parseOutputLevels(levelsStr string, minLevel zapcore.Level) []zapcore.Level {
	if levelsStr == "" {
		return nil // 空表示输出所有级别
	}

	levelsStr = strings.ToLower(strings.TrimSpace(levelsStr))
	if levelsStr == "all" {
		return nil // all 表示输出所有级别
	}

	var levels []zapcore.Level
	parts := strings.Split(levelsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "debug":
			if minLevel <= zapcore.DebugLevel {
				levels = append(levels, zapcore.DebugLevel)
			}
		case "info":
			if minLevel <= zapcore.InfoLevel {
				levels = append(levels, zapcore.InfoLevel)
			}
		case "warn", "warning":
			if minLevel <= zapcore.WarnLevel {
				levels = append(levels, zapcore.WarnLevel)
			}
		case "error":
			if minLevel <= zapcore.ErrorLevel {
				levels = append(levels, zapcore.ErrorLevel)
			}
		case "fatal":
			if minLevel <= zapcore.FatalLevel {
				levels = append(levels, zapcore.FatalLevel)
			}
		case "panic":
			if minLevel <= zapcore.PanicLevel {
				levels = append(levels, zapcore.PanicLevel)
			}
		}
	}
	return levels
}

// containsLevel 检查级别是否在列表中，如果列表为空表示包含所有级别
// 对于日志级别，如果配置了某个级别，则包含该级别及以上的所有级别
// 例如：配置了 error(2)，则 error(2)、fatal(3)、panic(4) 都包含
// 注意：zapcore 中级别数值越大，级别越高（DebugLevel=-1, InfoLevel=0, WarnLevel=1, ErrorLevel=2, FatalLevel=3, PanicLevel=4）
func containsLevel(levels []zapcore.Level, lvl zapcore.Level) bool {
	if len(levels) == 0 {
		return true // 空列表表示包含所有级别
	}
	for _, level := range levels {
		// 如果配置的级别 <= 当前级别，则包含（级别数值越大，级别越高）
		// 例如：配置了 error(2)，则 error(2)、fatal(3)、panic(4) 都包含
		if level <= lvl {
			return true
		}
	}
	return false
}

// containsExactLevel 检查级别是否精确匹配列表中的某个级别（用于分别输出模式）
// 如果列表为空表示包含所有级别
func containsExactLevel(levels []zapcore.Level, lvl zapcore.Level) bool {
	if len(levels) == 0 {
		return true // 空列表表示包含所有级别
	}
	for _, level := range levels {
		if level == lvl {
			return true
		}
	}
	return false
}

// buildFileCores 构建文件输出的 cores，支持分别输出到不同级别的文件
func buildFileCores(encoder zapcore.Encoder, wrapWriter func(io.Writer) zapcore.WriteSyncer,
	logFileName string, logConfig map[string]string, minLevel zapcore.Level, fileLevels []zapcore.Level) []zapcore.Core {

	var cores []zapcore.Core

	// 定义级别配置
	levelConfigs := []struct {
		level     zapcore.Level
		levelName string
		enabler   zap.LevelEnablerFunc
		shouldAdd bool
	}{
		{
			level:     zapcore.DebugLevel,
			levelName: "debug",
			enabler: zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= minLevel && lvl >= zapcore.DebugLevel && lvl < zapcore.InfoLevel
			}),
			shouldAdd: minLevel <= zapcore.DebugLevel,
		},
		{
			level:     zapcore.InfoLevel,
			levelName: "info",
			enabler: zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= minLevel && lvl >= zapcore.InfoLevel && lvl < zapcore.WarnLevel
			}),
			shouldAdd: minLevel <= zapcore.InfoLevel,
		},
		{
			level:     zapcore.WarnLevel,
			levelName: "warn",
			enabler: zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= minLevel && lvl >= zapcore.WarnLevel && lvl < zapcore.ErrorLevel
			}),
			shouldAdd: minLevel <= zapcore.WarnLevel,
		},
		{
			level:     zapcore.ErrorLevel,
			levelName: "error",
			enabler: zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= minLevel && lvl >= zapcore.ErrorLevel
			}),
			shouldAdd: minLevel <= zapcore.ErrorLevel,
		},
	}

	// 为每个级别创建 core
	for _, cfg := range levelConfigs {
		if !cfg.shouldAdd {
			continue
		}

		// 如果配置了 fileLevels，检查该级别是否需要输出
		// 在分别输出模式下，使用精确匹配，只创建用户明确配置的级别文件
		if len(fileLevels) > 0 && !containsExactLevel(fileLevels, cfg.level) {
			continue
		}

		writer := getWriter(fmt.Sprintf("./logs/%s_%s.log", logFileName, cfg.levelName), logConfig)
		cores = append(cores, zapcore.NewCore(encoder, wrapWriter(writer), cfg.enabler))
	}

	return cores
}
