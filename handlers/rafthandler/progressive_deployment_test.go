package rafthandler_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexandrestein/securelink/handlers/rafthandler"

	"github.com/alexandrestein/securelink"
)

func TestProgressiveDeploymentStartTreeNodesAndChangeLeader(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	server1, handler1 := buildHandler(t, ca, 1)
	defer server1.Close()

	server2, handler2 := buildHandler(t, ca, 2)
	defer server2.Close()

	server3, handler3 := buildHandler(t, ca, 3)
	defer server3.Close()

	err := handler1.Raft.AddPeer(rafthandler.MakePeerFromServer(server2))
	if err != nil {
		t.Fatal(err)
	}
	err = handler1.Raft.AddPeer(rafthandler.MakePeerFromServer(server3))
	if err != nil {
		t.Fatal(err)
	}

	err = handler1.Raft.Start()
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		for i, s := range []*securelink.Server{server1, server2, server3} {
			t.Logf("server %d has Uint64 ID as %d and Hex ID as %x", i+1, s.ID().Uint64(), s.ID().Bytes())
		}
	}

	time.Sleep(time.Second * 5)

	// Change the leader
	statusFrom1 := handler1.Raft.Node.Status()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	newLeader := uint64(0)
	if statusFrom1.Lead == handler1.Raft.ID.Uint64() {
		if testing.Verbose() {
			t.Logf("the leader is server 1 with Uint64 ID as %d and Hex ID as %x", handler1.Raft.ID.Uint64(), handler1.Raft.ID.Bytes())
		}
		newLeader = handler3.Raft.ID.Uint64()
		handler3.Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	} else if statusFrom1.Lead == handler2.Raft.ID.Uint64() {
		if testing.Verbose() {
			t.Logf("the leader is server 2 with Uint64 ID as %d and Hex ID as %x", handler2.Raft.ID.Uint64(), handler2.Raft.ID.Bytes())
		}
		newLeader = handler3.Raft.ID.Uint64()
		handler3.Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	} else {
		if testing.Verbose() {
			t.Logf("the leader is server 3 with Uint64 ID as %d and Hex ID as %x", handler3.Raft.ID.Uint64(), handler3.Raft.ID.Bytes())
		}
		newLeader = handler2.Raft.ID.Uint64()
		handler2.Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	}

	// fmt.Println("handler1.Raft.Node.Status()", handler1.Raft.Node.Status().RaftState, handler1.Raft.Node.Status())
	// bs := make([]byte, 8)
	// binary.BigEndian.PutUint64(bs, handler1.Raft.Node.Status().Lead)
	// fmt.Printf("handler1.Raft.Node.Lead() %x\n", bs)
	// fmt.Println("handler1.Raft.Node.Status()", handler1.Raft.Node.Status().RaftState)
	// fmt.Println("handler2.Raft.Node.Status()", handler2.Raft.Node.Status().RaftState)
	// fmt.Println("handler3.Raft.Node.Status()", handler3.Raft.Node.Status().RaftState)

	time.Sleep(time.Second * 5)

	if newLeader != handler1.Raft.Node.Status().Lead ||
		newLeader != handler2.Raft.Node.Status().Lead ||
		newLeader != handler3.Raft.Node.Status().Lead {
		t.Fatalf("the leader ship has not been moved correctly\n\tTaget was %d\n\t1 has for leader %d\n\t2 has for leader %d\n\t3 has for leader %d",
			newLeader, handler1.Raft.Node.Status().Lead, handler2.Raft.Node.Status().Lead, handler3.Raft.Node.Status().Lead,
		)
	}

	if testing.Verbose() {
		statusFrom1 = handler1.Raft.Node.Status()
		t.Logf("the leader new Uint64 ID as %d", statusFrom1.Lead)
	}
}
