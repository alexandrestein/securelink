package rafthandler

import (
	"github.com/labstack/gommon/log"
)

type (
	// Logger implements the raft logger interface to be able to hide most of
	// the output from raft
	Logger struct {
		*log.Logger
	}
)

// NewLogger build a new logger
func NewLogger(id string, level log.Lvl) *Logger {
	logger := log.New(id)
	logger.SetLevel(level)
	header := "${time_rfc3339} ${level} => ${prefix} -> ${short_file}:${line}"
	logger.SetHeader(header)

	ret := &Logger{
		Logger: logger,
	}
	return ret
}

// Warning implements the raft logger interface
func (l *Logger) Warning(v ...interface{}) {
	l.Warn(v...)
}

// Warningf implements the raft logger interface
func (l *Logger) Warningf(format string, v ...interface{}) {
	l.Warnf(format, v...)
}
