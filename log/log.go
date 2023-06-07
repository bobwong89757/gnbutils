package log

import (
	"fmt"
	"github.com/bobwong89757/golog/logs"
	"strings"
)

var MLog = &Log{}

type Log struct {
	logger *logs.BeeLogger
}

func (l *Log) InitLog(logConfig map[string]string, logFileName string) {
	// l.logger.Reset()
	l.logger = logs.NewLogger()
	l.logger.SetLogger(logConfig["type"], strings.Replace(logConfig["config"], "default.log", fmt.Sprintf("%s.log", logFileName), -1))
}

func (l *Log) GetLog() *logs.BeeLogger {
	return l.logger
}
