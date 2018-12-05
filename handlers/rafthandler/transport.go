package rafthandler

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/raft"
)

func (t *Transport) Accept() (net.Conn, error) {
	conn, ok := <-t.AcceptChan
	if !ok {
		return nil, fmt.Errorf("looks close")
	}
	return conn, nil
}

func (t *Transport) Addr() net.Addr {
	return t.Server.Addr()
}

func (t *Transport) Close() error {
	return t.Server.Close()
}

func (t *Transport) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	return t.Server.Dial(string(address), HostPrefix, timeout)
}
