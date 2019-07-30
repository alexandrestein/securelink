package raft

import (
	"fmt"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/coreos/etcd/raft"
	pb "github.com/coreos/etcd/raft/raftpb"
)

type (
	Node struct {
		// *raft.RawNode
		raft.Node

		Server *securelink.Server
	}
)

func StartNode(s *securelink.Server, peerIDs []uint64) (*Node, error) {
	storage := raft.NewMemoryStorage()
	c := &raft.Config{
		ID:              s.Certificate.ID().Uint64(),
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	raftPeers := make([]raft.Peer, len(peerIDs))
	for i, peerID := range peerIDs {
		raftPeers[i] = raft.Peer{ID: peerID}
	}

	// node, err := raft.NewRawNode(c, raftPeers)
	// if err != nil {
	// 	return nil, err
	// }

	n := &Node{
		// node,
		raft.StartNode(c, raftPeers),
		s,
	}

	go n.loop()

	return n, nil
}

func (n *Node) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.Tick()
		case rd := <-n.Ready():
			fmt.Println("ready")
			saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)
			send(rd.Messages)
			if !raft.IsEmptySnap(rd.Snapshot) {
				processSnapshot(rd.Snapshot)
			}
			for _, entry := range rd.CommittedEntries {
				process(entry)
				if entry.Type == pb.EntryConfChange {
					var cc pb.ConfChange
					cc.Unmarshal(entry.Data)
					n.ApplyConfChange(cc)
				}
			}
			n.Advance()
		case <-n.Server.Ctx().Done():
			return
		}
	}
}

func saveToStorage(state pb.HardState, entries []pb.Entry, snap pb.Snapshot) {
	fmt.Println("save", state, entries, snap)
}

func send(msgs []pb.Message) {
	fmt.Println("send", msgs)

	for i, msg := range msgs {
		fmt.Printf("msg %d: %v", i, msg)
	}
}

func processSnapshot(snap pb.Snapshot) {
	fmt.Println("snap", snap)
}

func process(entry pb.Entry) {
	fmt.Println("process", entry)
}
