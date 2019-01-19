package log

import (
	"github.com/astaxie/beego/logs"
)

var MLog = &Log{}

type Log struct {
	logger *logs.BeeLogger
}

func (l *Log) InitLog(logConfig map[string]string) {
	l.logger.Reset()
	l.logger = logs.NewLogger()
	l.logger.SetLogger(logConfig["type"], logConfig["config"])
}

func (l *Log) GetLog() *logs.BeeLogger {
	return l.logger
}
