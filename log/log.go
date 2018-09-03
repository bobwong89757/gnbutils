package log

import (
	"github.com/astaxie/beego/logs"
)

var MLog = &Log{}

type Log struct {
	logger *logs.BeeLogger
}

func (l *Log) InitLog(logConfig map[string]string) {
	l.logger = logs.NewLogger()
	logs.SetLogger(logConfig["type"], logConfig["config"])
	l.logger.Info("===>>>> init log success")
}

func (l *Log) GetLog() *logs.BeeLogger {
	return l.logger
}
