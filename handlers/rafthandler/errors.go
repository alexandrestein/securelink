package rafthandler

import "fmt"

// Those variables defines the different errors from this package
var (
	ErrNotEnoughNodesForRaftToStart = fmt.Errorf("can't start raft with less than 3 nodes")
	ErrRaftNodeNotLoaded            = fmt.Errorf("the raft node is not started wet")
	ErrRaftNodeStarted              = fmt.Errorf("the raft node is already started")
	ErrBadResponseCode              = func(code int) error {
		return fmt.Errorf("the response code is not between 200 included and 300 exclude but: %d", code)
	}
)
