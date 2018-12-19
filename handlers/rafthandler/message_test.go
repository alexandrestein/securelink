package rafthandler_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alexandrestein/securelink/handlers/rafthandler"
	"github.com/etcd-io/etcd/raft"
)

func TestMessage(t *testing.T) {
	nbServers := 5
	nbTestMessage := 15
	waitingDuration := time.Second * 2

	servers, handlers := startNServer(t, nbServers)
	defer closeServers(servers)

	time.Sleep(waitingDuration)

	var leader, nonLeader *rafthandler.Handler
	for _, h := range handlers {
		if h.Raft.Node.Status().SoftState.RaftState == raft.StateLeader {
			leader = h
		} else {
			nonLeader = h
		}

		if leader != nil && nonLeader != nil {
			break
		}
	}
	for _, handler := range handlers {
		go waitForMessages(t, waitingDuration, nbTestMessage, handler.Raft.Messages, handler.Transport.ID().String())
	}

	for i := 0; i < nbTestMessage; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), waitingDuration)
		defer cancel()
		err := nonLeader.Raft.Node.Propose(ctx, []byte(fmt.Sprintf("%d", i)))
		if err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(waitingDuration)
}

func waitForMessages(t *testing.T, before time.Duration, n int, ch chan []byte, id string) {
	received := make([]int64, n)
	deadline := time.After(before)
	for i := range received {
		select {
		case message := <-ch:
			receivedInt, err := strconv.ParseInt(string(message), 10, 8)
			if err != nil {
				t.Fatal(err)
			}
			if i != int(receivedInt) {
				t.Fatalf("expected value %d but had %d", i, receivedInt)
			}
			received[i] = receivedInt
		case <-deadline:
			t.Fatalf("not completed before the deadline %v", received)
		}
	}

	if testing.Verbose() {
		t.Logf("completed for %q with: %v", id, received)
	}
}
