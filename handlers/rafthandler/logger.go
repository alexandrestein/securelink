package rafthandler

import (
	"github.com/etcd-io/etcd/raft"
	"github.com/labstack/gommon/log"
)

type (
	errLogger struct {
		*log.Logger
	}
)

func newErrLogger(id string) raft.Logger {
	logger := log.New(id)
	logger.SetLevel(log.ERROR)

	ret := &errLogger{
		Logger: logger,
	}
	return ret
}

func (l *errLogger) Warning(v ...interface{}) {
	l.Warn(v...)
}
func (l *errLogger) Warningf(format string, v ...interface{}) {
	l.Warnf(format, v...)
}
