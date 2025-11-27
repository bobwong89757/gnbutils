package log

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
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
	l.redirectStderrToLog(logFileName)
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

// redirectStderrToLog 重定向标准错误到日志文件
// Go 运行时的 fatal error（如并发 map 访问错误）会直接输出到 stderr，
// 无法通过 recover() 捕获，因此需要重定向 stderr 到日志文件以确保这些错误被记录
//
// 实现方式：使用文件描述符重定向，将 stderr 的输出同时写入日志文件和终端
//
// 注意：正常情况下 stderr 日志文件应该是空的或很少内容，只有以下情况才会写入：
//
//   - Go 运行时的 fatal error（如 fatal error: concurrent map read and map write）
//
//   - Panic 信息
//
//   - Race detector 报告（如果使用 -race 编译）
//
//   - 其他运行时错误
//
//     @Description: 重定向 stderr 到日志文件
//     @param procName 进程名称，用于生成日志文件名
func (l *Log) redirectStderrToLog(procName string) {
	// 获取日志目录（通常与主日志文件在同一目录）
	// 尝试从配置中获取日志路径，如果没有则使用默认路径
	logDir := "./logs"

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// 如果创建目录失败，使用当前目录
		logDir = "."
	}

	// 创建 stderr 日志文件，文件名包含进程名和日期
	// 使用日期作为文件名，同一天的所有 fatal error 会追加到同一个文件
	dateStr := time.Now().Format("20060102")
	stderrLogFile := filepath.Join(logDir, procName+"_stderr_"+dateStr+".log")

	// 打开文件（追加模式，如果文件不存在则创建）
	// 如果文件已存在，会自动追加到文件末尾，不会覆盖原有内容
	file, err := os.OpenFile(stderrLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 如果打开文件失败，记录警告但不影响程序运行
		l.GetLog().Warnf("无法创建 stderr 日志文件 %s: %v，fatal error 将不会记录到日志文件", stderrLogFile, err)
		return
	}

	// 创建一个管道来拦截 stderr 的输出
	reader, writer, err := os.Pipe()
	if err != nil {
		l.GetLog().Warnf("无法创建管道重定向 stderr: %v", err)
		file.Close()
		return
	}

	// 保存原始的 stderr 文件描述符（在重定向前）
	originalStderrFd := int(os.Stderr.Fd())

	// 将 stderr 重定向到管道的写入端
	err = syscall.Dup2(int(writer.Fd()), originalStderrFd)
	if err != nil {
		l.GetLog().Warnf("无法重定向 stderr 文件描述符: %v，fatal error 可能不会记录到日志文件", err)
		reader.Close()
		writer.Close()
		file.Close()
		return
	}

	// 创建带缓冲的写入器，确保数据能及时刷新到磁盘
	// 使用较小的缓冲区（4KB），确保 fatal error 能及时写入
	bufferedFileWriter := bufio.NewWriterSize(file, 4096)

	// 创建最终的写入器（如果是终端+文件，需要同时写入）
	var finalWriter io.Writer = bufferedFileWriter
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		// 同时写入终端和文件（带缓冲）
		finalWriter = io.MultiWriter(tty, bufferedFileWriter)
		// 注意：不要关闭 tty，因为我们需要持续写入
	} else {
		// 无法打开终端（可能是后台运行），只写入文件
		l.GetLog().Debugf("无法打开终端设备 /dev/tty: %v，stderr 将只写入日志文件", err)
	}

	// 启动一个 goroutine 来读取管道内容并写入文件和终端
	go func() {
		// 添加 panic recovery，防止 stderr 重定向 goroutine 崩溃
		defer func() {
			if r := recover(); r != nil {
				// 如果 goroutine panic，尝试直接写入文件
				file.WriteString("stderr 重定向 goroutine panic: " + string(debug.Stack()) + "\n")
				file.Sync() // 立即同步到磁盘
			}
		}()

		defer reader.Close()
		defer writer.Close()
		// 注意：不要在这里关闭 file，因为我们需要持续写入
		// 文件会在程序退出时自动关闭

		// 使用带缓冲的写入，并定期刷新
		buf := make([]byte, 4096)
		ticker := time.NewTicker(100 * time.Millisecond) // 每100ms刷新一次
		defer ticker.Stop()

		// 启动一个 goroutine 定期刷新缓冲区
		go func() {
			for range ticker.C {
				bufferedFileWriter.Flush()
				file.Sync() // 同步到磁盘
			}
		}()

		// 读取管道内容并写入
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				// 写入数据
				if _, writeErr := finalWriter.Write(buf[:n]); writeErr != nil {
					// 写入失败，尝试直接写入文件（绕过缓冲）
					file.Write(buf[:n])
					file.Sync()
				} else {
					// 写入成功，立即刷新缓冲区（确保 fatal error 能及时写入）
					bufferedFileWriter.Flush()
					file.Sync() // 同步到磁盘
				}
			}
			if err != nil {
				if err != io.EOF {
					// 读取错误，记录到文件
					file.WriteString("stderr 重定向读取错误: " + err.Error() + "\n")
					file.Sync()
				}
				break
			}
		}

		// 退出前最后刷新一次
		bufferedFileWriter.Flush()
		file.Sync()
	}()

	// 写入启动标记，确认重定向成功（立即刷新）
	startTime := time.Now().Format("2006-01-02 15:04:05")
	marker := "=== stderr 重定向启动 [" + startTime + "] ===\n"
	file.WriteString(marker)
	file.Sync() // 立即同步，确保标记被写入

	// 记录重定向成功的信息
	l.GetLog().Infof("stderr 已重定向到日志文件: %s", stderrLogFile)
}
