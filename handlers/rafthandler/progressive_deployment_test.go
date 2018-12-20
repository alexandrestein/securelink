package rafthandler_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexandrestein/securelink/handlers/rafthandler"

	"github.com/alexandrestein/securelink"
)

func TestProgressiveDeployment(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	servers, handlers := []*securelink.Server{}, []*rafthandler.Handler{}
	t.Run("start 3 nodes", func(t *testing.T) {
		servers, handlers = testStart3Nodes(t, ca)
	})
	defer closeServers(servers)

	time.Sleep(time.Second * 4)

	t.Run("update leader", func(t *testing.T) {
		testUpdateLeader(t, handlers)
	})

	time.Sleep(time.Second * 2)

	t.Run("update add 5 nodes", func(t *testing.T) {
		serversBis, handlersBis := testAdd5Nodes(t, ca, handlers[0])
		servers = append(servers, serversBis...)
		handlers = append(handlers, handlersBis...)
	})

	time.Sleep(time.Second * 4)
}

func testStart3Nodes(t *testing.T, ca *securelink.Certificate) (servers []*securelink.Server, handlers []*rafthandler.Handler) {
	server1, handler1 := buildHandler(t, ca, 1)

	server2, handler2 := buildHandler(t, ca, 2)

	server3, handler3 := buildHandler(t, ca, 3)

	err := handler1.Raft.AddPeer(rafthandler.MakePeerFromServer(server2))
	if err != nil {
		t.Fatal(err)
	}
	err = handler1.Raft.AddPeer(rafthandler.MakePeerFromServer(server3))
	if err != nil {
		t.Fatal(err)
	}

	err = handler1.Raft.Start(false)
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		for i, s := range []*securelink.Server{server1, server2, server3} {
			t.Logf("server %d has Uint64 ID as %d and Hex ID as %x", i+1, s.ID().Uint64(), s.ID().Bytes())
		}
	}

	servers = append(servers, server1, server2, server3)
	handlers = append(handlers, handler1, handler2, handler3)

	return
}

func testUpdateLeader(t *testing.T, handlers []*rafthandler.Handler) {
	statusFrom1 := handlers[0].Raft.Node.Status()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	newLeader := uint64(0)
	if statusFrom1.Lead == handlers[0].Raft.ID.Uint64() {
		if testing.Verbose() {
			t.Logf("the leader was server 1 with Uint64 ID as %d and Hex ID as %x", handlers[0].Raft.ID.Uint64(), handlers[0].Raft.ID.Bytes())
		}
		newLeader = handlers[2].Raft.ID.Uint64()
		handlers[2].Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	} else if statusFrom1.Lead == handlers[1].Raft.ID.Uint64() {
		if testing.Verbose() {
			t.Logf("the leader was server 2 with Uint64 ID as %d and Hex ID as %x", handlers[1].Raft.ID.Uint64(), handlers[1].Raft.ID.Bytes())
		}
		newLeader = handlers[2].Raft.ID.Uint64()
		handlers[2].Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	} else {
		if testing.Verbose() {
			t.Logf("the leader was server 3 with Uint64 ID as %d and Hex ID as %x", handlers[2].Raft.ID.Uint64(), handlers[2].Raft.ID.Bytes())
		}
		newLeader = handlers[1].Raft.ID.Uint64()
		handlers[1].Raft.Node.TransferLeadership(ctx, statusFrom1.Lead, newLeader)
	}

	time.Sleep(time.Second * 3)

	if newLeader != handlers[0].Raft.Node.Status().Lead ||
		newLeader != handlers[1].Raft.Node.Status().Lead ||
		newLeader != handlers[2].Raft.Node.Status().Lead {
		t.Fatalf("the leader ship has not been moved correctly\n\tTaget was %d\n\t1 has for leader %d\n\t2 has for leader %d\n\t3 has for leader %d",
			newLeader, handlers[0].Raft.Node.Status().Lead, handlers[1].Raft.Node.Status().Lead, handlers[2].Raft.Node.Status().Lead,
		)
	}

	if testing.Verbose() {
		statusFrom1 = handlers[0].Raft.Node.Status()
		t.Logf("the new leader ID as Uint64 %d", statusFrom1.Lead)
	}
}

func testAdd5Nodes(t *testing.T, ca *securelink.Certificate, livingNode *rafthandler.Handler) (servers []*securelink.Server, handlers []*rafthandler.Handler) {
	servers = make([]*securelink.Server, 5)
	handlers = make([]*rafthandler.Handler, 5)
	servers[0], handlers[0] = buildHandler(t, ca, 4)
	servers[1], handlers[1] = buildHandler(t, ca, 5)
	servers[2], handlers[2] = buildHandler(t, ca, 6)
	servers[3], handlers[3] = buildHandler(t, ca, 7)
	servers[4], handlers[4] = buildHandler(t, ca, 8)

	for _, server := range servers {
		rf := rafthandler.MakePeerFromServer(server)
		err := livingNode.Raft.AddPeer(rf)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Second * 2)
	}

	if nbNodes := len(livingNode.Raft.Node.Status().Progress); nbNodes != 8 {
		t.Fatalf("expected 8 nodes but had %d", nbNodes)
	}

	return
}
