package rafthandler

import (
	"github.com/labstack/gommon/log"
)

type (
	Logger struct {
		*log.Logger
	}
)

func NewLogger(id string, level log.Lvl) *Logger {
	logger := log.New(id)
	logger.SetLevel(level)
	header := "${time_rfc3339} ${level} => ${prefix}->${short_file}:${line}"
	logger.SetHeader(header)

	ret := &Logger{
		Logger: logger,
	}
	return ret
}

func (l *Logger) Warning(v ...interface{}) {
	l.Warn(v...)
}
func (l *Logger) Warningf(format string, v ...interface{}) {
	l.Warnf(format, v...)
}
