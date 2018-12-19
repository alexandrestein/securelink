package rafthandler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net"
	"regexp"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/echohandler"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	"github.com/labstack/gommon/log"
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
	AddNode    = "/addNode"
	StartNodes = "/start"
	Message    = "/message"
)

var (
	matchRaftPrefixRegexp     = regexp.MustCompile(fmt.Sprintf(`^%s\.`, HostPrefix))
	matchRaftPrefixRegexpFunc = func(s string) bool {
		return matchRaftPrefixRegexp.MatchString(s)
	}
)

type (
	Handler struct {
		*echohandler.Handler

		Server *securelink.Server
		Raft   *Raft

		Transport *Transport

		Logger *Logger
	}

	Raft struct {
		ID     *big.Int
		Node   raft.Node
		Ticker *time.Ticker
		done   chan struct{}

		Transport *Transport

		Storage *raft.MemoryStorage

		Logger *Logger

		Messages chan []byte
	}

	Transport struct {
		*securelink.Server

		EchoHandler *echohandler.Handler
		Peers       *Peers
	}
)

func New(addr net.Addr, name string, server *securelink.Server, logger *Logger) (*Handler, error) {
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
	ret.Transport.Peers.AddPeers(MakePeerFromServer(server))

	ret.Logger = logger

	ret.Server = server

	err = ret.initEcho()
	if err != nil {
		return nil, err
	}

	ret.Raft = new(Raft)
	ret.Raft.ID = ret.Server.ID()
	ret.Raft.Storage = raft.NewMemoryStorage()
	ret.Raft.Transport = ret.Transport
	ret.Raft.Logger = ret.Logger
	ret.Raft.Messages = make(chan []byte, 16)

	go ret.Transport.EchoHandler.Start()

	server.RegisterService(ret)

	return ret, nil
}

func (r *Raft) Start() (err error) {
	// If started no need to start the node again.
	// This will be called multiple times at the startup. Every nodes which get
	// a start signal will broadcast it to all know hosts.
	// This will prevent initializing loops.
	if r.Node != nil {
		return nil
	}

	if r.Transport.Peers.Len() < 3 {
		return ErrNotEnoughNodesForRaftToStart
	}

	if r.Logger == nil {
		r.Logger = NewLogger(r.Transport.ID().String(), log.ERROR)
	}

	c := &raft.Config{
		ID:              r.ID.Uint64(),
		ElectionTick:    20,
		HeartbeatTick:   2,
		Storage:         r.Storage,
		MaxSizePerMsg:   math.MaxUint64,
		MaxInflightMsgs: 256,
		CheckQuorum:     true,
		PreVote:         true,
		Logger:          r.Logger,
	}

	r.Ticker = time.NewTicker(time.Millisecond * 50)
	r.done = make(chan struct{})

	r.Node = raft.StartNode(c, r.Transport.Peers.ToRaftPeers())
	go r.raftLoop()

	err = r.Transport.HeadToAll(StartNodes, 0)
	if err != nil {
		r.Node.Stop()
		return err
	}

	return nil
}

func (r *Raft) AddPeer(peer *Peer) error {
	if peer.ID == r.ID.Uint64() {
		return nil
	}

	r.Transport.Peers.AddPeers(peer)

	bytes, err := json.Marshal(r.Transport.Peers.GetPeers())
	if err != nil {
		return err
	}

	err = r.Transport.PostJSONToAll(AddNode, bytes, DefaultRequestTimeOut)
	return err
}

func (r *Raft) LocalPeer() *Peer {
	return MakePeer(r.Transport.ID().Uint64(), r.Transport.AddrStruct)
}

func (r *Raft) raftLoop() {
	for {
		select {
		case <-r.Ticker.C:
			r.Node.Tick()
		case rd := <-r.Node.Ready():
			r.saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)
			r.send(rd.Messages)
			if !raft.IsEmptySnap(rd.Snapshot) {
				r.processSnapshot(rd.Snapshot)
			}
			for _, entry := range rd.CommittedEntries {
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
	r.Storage.Append(entries)

	if !raft.IsEmptyHardState(hardState) {
		r.Storage.SetHardState(hardState)
	}

	if !raft.IsEmptySnap(snapshot) {
		r.Storage.ApplySnapshot(snapshot)
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
	// fmt.Println("process", raft.DescribeEntry(entry, nil))
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		r.Messages <- entry.Data

		// parts := bytes.SplitN(entry.Data, []byte(":"), 2)
		// r.pstore[string(parts[0])] = string(parts[1])
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

func (r *Raft) StopRaft() {
	r.Node.Stop()
	r.Ticker.Stop()
	r.done <- struct{}{}
	r.Node = nil
}

func (h *Handler) Close() {
	h.Raft.StopRaft()
	close(h.Raft.Messages)
	h.Handler.Close()
	h.Server.DeregisterService(h.Name())
}

func (r *Raft) addNode(peers ...*Peer) error {
	r.Transport.Peers.AddPeers(peers...)

	if r.Node == nil {
		return nil
	}

	for _, peer := range peers {
		if peer.ID == r.ID.Uint64() {
			continue
		}

		cc := raftpb.ConfChange{
			Type:   raftpb.ConfChangeAddNode,
			NodeID: peer.ID,
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		r.Node.ApplyConfChange(cc)
		return r.Node.ProposeConfChange(ctx, cc)
	}
	return nil
}
