package log

import (
	"fmt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"strings"
	"time"
)

var MLog = &Log{}

type Log struct {
	logger *zap.SugaredLogger
}

func (l *Log) InitLog(logConfig map[string]string, logFileName string) {

	levelEncoder := zapcore.CapitalLevelEncoder
	useColor, ok := logConfig["color"]
	if ok {
		if strings.EqualFold(useColor, "true") {
			levelEncoder = zapcore.CapitalColorLevelEncoder
		}
	}

	logType, _ := logConfig["type"]

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
	// 实现两个判断日志等级的interface
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.InfoLevel
	})

	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	// 获取 info、error日志文件的io.Writer 抽象 getWriter() 在下方实现
	infoWriter := getWriter(fmt.Sprintf("./logs/%s_info.log", logFileName))
	errorWriter := getWriter(fmt.Sprintf("./logs/%s_error.log", logFileName))

	// 最后创建具体的Logger
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), infoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), errorLevel),
	)
	if strings.EqualFold(logType, "console") {
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), infoLevel),
			zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), errorLevel),
		)
	}
	if strings.EqualFold(logType, "file") {
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel),
			zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel),
		)
	}
	if strings.EqualFold(logType, "hybrid") {
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), infoLevel),
			zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), errorLevel),
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel),
			zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel),
		)
	}

	// 需要传入 zap.AddCaller() 才会显示打日志点的文件名和行数
	log := zap.New(core, zap.AddCaller())
	l.logger = log.Sugar()
}

func (l *Log) GetLog() *zap.SugaredLogger {
	return l.logger
}

func getWriter(filename string) io.Writer {
	// 生成rotatelogs的Logger 实际生成的文件名 demo.log.YYmmddHH
	// demo.log是指向最新日志的链接
	// 保存7天内的日志，每1小时(整点)分割一次日志
	hook, err := rotatelogs.New(
		strings.Replace(filename, ".log", "", -1)+"-%Y%m%d%H.log", // 没有使用go风格反人类的format格式
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithMaxAge(time.Hour*24*7),
		rotatelogs.WithRotationTime(time.Hour),
	)

	if err != nil {
		panic(err)
	}
	return hook
}
