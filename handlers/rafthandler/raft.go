package rafthandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"regexp"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/echohandler"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
)

const (
	// HostPrefix defines the prefix used to define the service.
	// If the node id is XXX the host name provided during handshake will be
	// <HostPrefix>.XXX.
	HostPrefix = "raft"
	// // HostHTTPPrefix is same as above but for HTTP splitter
	// HostHTTPPrefix = "http-raft"
)

// Defines the paths for the HTTP API
const (
	// GetServerInfo = "/"
	AddNode = "/addNode"
	Message = "/message"
)

var (
	matchRaftPrefixRegexp     = regexp.MustCompile(fmt.Sprintf(`^%s\.`, HostPrefix))
	matchRaftPrefixRegexpFunc = func(s string) bool {
		return matchRaftPrefixRegexp.MatchString(s)
	}
)

type (
	Handler struct {
		securelink.Handler

		Server *securelink.Server
		Raft   *Raft

		Transport *Transport
	}

	Raft struct {
		ID     *big.Int
		Node   raft.Node
		Ticker *time.Ticker
		done   chan struct{}

		Transport *Transport

		storage *raft.MemoryStorage

		// pstore is a fake implementation of a persistent storage
		// that will be used side-by-side with the WAL in the raft
		pstore map[string]string
	}

	// Storage interface {
	// 	raft.LogStore
	// 	raft.StableStore
	// 	raft.SnapshotStore
	// }

	Transport struct {
		*securelink.Server
		// *securelink.BaseListener

		EchoHandler *echohandler.Handler
		Peers       *Peers
	}

	// conn struct {
	// 	net.Conn
	// 	// wg sync.WaitGroup
	// }
)

func New(addr net.Addr, name string, server *securelink.Server) (*Handler, error) {
	ret := new(Handler)
	echoHandler, err := echohandler.New(addr, HostPrefix, server.TLS.Config)
	if err != nil {
		return nil, err
	}

	ret.Handler = echoHandler

	ret.Transport = new(Transport)
	ret.Transport.Server = server
	ret.Transport.EchoHandler = echoHandler
	ret.Transport.Peers = NewPeers()

	ret.Server = server

	err = ret.initEcho()
	if err != nil {
		return nil, err
	}

	ret.Raft = new(Raft)
	ret.Raft.ID = ret.Server.ID()
	ret.Raft.storage = raft.NewMemoryStorage()
	ret.Raft.Transport = ret.Transport
	ret.Raft.pstore = map[string]string{}

	return ret, nil
}

func (r *Raft) Start() (err error) {
	id := r.ID.Uint64()
	c := &raft.Config{
		ID:              id,
		ElectionTick:    5,
		HeartbeatTick:   2,
		Storage:         r.storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	// r.Ticker = time.NewTicker(time.Second * 10)
	r.Ticker = time.NewTicker(time.Millisecond * 500)

	// raftPeers := []raft.Peer{}
	// if peers == nil {
	// 	r.Node = raft.StartNode(c, []raft.Peer{{ID: id}})
	// } else {
	// 	r.Node = raft.StartNode(c, peers)
	// }
	r.Node = raft.StartNode(c, r.Transport.Peers.ToRaftPeers())

	go r.raftLoop()
	go r.Transport.EchoHandler.Start()

	return err
}

func (r *Raft) AddPeer(peer *Peer) error {
	r.Transport.Peers.AddPeers(peer)

	bytes, err := json.Marshal(peer)
	if err != nil {
		return err
	}

	err = r.Transport.PostJSONToAll(AddNode, bytes, time.Second*5)
	return err
}

func (r *Raft) raftLoop() {
	for {
		select {
		case <-r.Ticker.C:
			r.Node.Tick()
		case rd := <-r.Node.Ready():
			// fmt.Println("saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)", rd.HardState, rd.Entries, rd.Snapshot)
			r.saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)
			// fmt.Println("send(rd.Messages)", rd.Messages)
			r.send(rd.Messages)
			if !raft.IsEmptySnap(rd.Snapshot) {
				// fmt.Println("processSnapshot(rd.Snapshot)", rd.Snapshot)
				r.processSnapshot(rd.Snapshot)
			}
			for _, entry := range rd.CommittedEntries {
				// fmt.Println("process(entry)", entry)
				r.process(entry)
				if entry.Type == raftpb.EntryConfChange {
					var cc raftpb.ConfChange
					cc.Unmarshal(entry.Data)
					r.Node.ApplyConfChange(cc)
				}
			}
			r.Node.Advance()
		case <-r.done:
			return
		}
	}
}

func (r *Raft) saveToStorage(hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) {
	r.storage.Append(entries)

	if !raft.IsEmptyHardState(hardState) {
		r.storage.SetHardState(hardState)
	}

	if !raft.IsEmptySnap(snapshot) {
		r.storage.ApplySnapshot(snapshot)
	}
}

func (r *Raft) send(messages []raftpb.Message) {
	for _, msg := range messages {
		msgAsBytes, err := msg.Marshal()
		if err != nil {
			continue
		}

		err = r.Transport.SendMessageTo(msg.To, msgAsBytes, 0)
		if err != nil {
			continue
		}
	}
}

func (r *Raft) processSnapshot(snapshot raftpb.Snapshot) {
	log.Printf("Applying snapshot is not implemented yet")
}

func (r *Raft) process(entry raftpb.Entry) {
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		log.Println("normal message:", string(entry.Data))

		parts := bytes.SplitN(entry.Data, []byte(":"), 2)
		r.pstore[string(parts[0])] = string(parts[1])
	}
}

// func (r *Raft) HandleMessage(conn net.Conn) {
// 	buf := make([]byte, 4096)
// 	n, err := conn.Read(buf)
// 	if err != nil {
// 		fmt.Println("r *Raft) HandleMessage", 0, err)
// 		return
// 	}

// 	buf = buf[:n]
// 	msg := raftpb.Message{}
// 	err = msg.Unmarshal(buf)
// 	if err != nil {
// 		fmt.Println("r *Raft) HandleMessage", 1, err)
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*750)
// 	defer cancel()
// 	err = r.Node.Step(ctx, msg)
// 	if err != nil {
// 		fmt.Println("r *Raft) HandleMessage", 2, err)
// 		return
// 	}
// }

func (r *Raft) Close() {
	r.Node.Stop()
	r.Ticker.Stop()
	r.done <- struct{}{}
}

// func (h *Handler) Handle(conn net.Conn) error {
// 	// cc := newConn(conn)

// 	fmt.Println("handle")

// 	h.Raft.HandleMessage(conn)

// 	// h.Transport.AcceptChan <- conn

// 	// cc.wg.Wait()

// 	return nil
// }

// func newConn(regConn net.Conn) *conn {
// 	ret := &conn{
// 		Conn: regConn,
// 		// wg:   sync.WaitGroup{},
// 	}

// 	// ret.wg.Add(1)

// 	return ret
// }

// func (r *Raft) AddNode(peer *Peer) error {
// 	r.Transport.Peers.AddPeers(peer)

// 	bytes, err := json.Marshal(peer)
// 	if err != nil {
// 		return err
// 	}

// 	err = r.Transport.PostJSONToAll(AddNode, bytes, time.Second*2)
// 	return err
// }

func (r *Raft) addNode(peer *Peer) error {
	r.Transport.Peers.AddPeers(peer)

	cc := raftpb.ConfChange{
		// ID:   peer.ID,
		Type:   raftpb.ConfChangeAddNode,
		NodeID: peer.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	r.Node.ApplyConfChange(cc)
	return r.Node.ProposeConfChange(ctx, cc)
}
