package rafthandler

import (
	"fmt"
	"net"
	"time"
)

func (t *Transport) Accept() (net.Conn, error) {
	fmt.Println("accept")
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

func (t *Transport) Dial(destID uint64, timeout time.Duration) (net.Conn, error) {
	for _, peer := range t.Peers {
		if peer.ID == destID {
			return t.Server.Dial(peer.String(), HostPrefix, timeout)
		}
	}

	return nil, fmt.Errorf("ID not found")
}

func (t *Transport) SendTo(destID uint64, message []byte) error {
	conn, err := t.Dial(destID, time.Second)
	if err != nil {
		return err
	}

	var n int
	n, err = conn.Write(message)
	if err != nil {
		return err
	}

	if n != len(message) {
		return fmt.Errorf("message not totally sent, missing %d", len(message)-n)
	}

	return nil
}
