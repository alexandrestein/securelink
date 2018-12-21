package rafthandler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
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
)

// Defines the paths for the HTTP API
const (
	// GetServerInfo = "/"
	AddNode = "/addNode"
	// RMNode      = "/rmNode"
	StartNodes  = "/start"
	JoinCluster = "/join"
	StopNodes   = "/stop"
	Message     = "/message"
)

var (
	matchRaftPrefixRegexp     = regexp.MustCompile(fmt.Sprintf(`^%s\.`, HostPrefix))
	matchRaftPrefixRegexpFunc = func(s string) bool {
		return matchRaftPrefixRegexp.MatchString(s)
	}
)

type (
	// Handler defines the main element of the package.
	// This holds every components for managing the raft cluster
	Handler struct {
		*echohandler.Handler

		Server *securelink.Server
		Raft   *Raft

		Transport *Transport

		Logger *Logger
	}

	// Raft holds the raft components
	Raft struct {
		ID      *big.Int
		Node    raft.Node
		Ticker  *time.Ticker
		done    chan struct{}
		applied uint64

		Transport *Transport

		Storage *raft.MemoryStorage

		Logger *Logger

		Messages chan []byte

		confState *raftpb.ConfState
	}

	// Transport is an sub element of the Handler ans Raft pointer
	Transport struct {
		*securelink.Server

		EchoHandler *echohandler.Handler
		Peers       *Peers
	}
)

// New builds a new raft handler on the given server.
//
// This function self register the handler on the server with the given name.
// Logger can be empty. A logger with WARN level is used instead.
func New(name string, server *securelink.Server, logger *Logger) (*Handler, error) {
	ret := new(Handler)
	echoHandler, err := echohandler.New(server.AddrStruct, HostPrefix, server.TLS.Config)
	if err != nil {
		return nil, err
	}

	ret.Handler = echoHandler

	ret.Transport = new(Transport)
	ret.Transport.Server = server
	ret.Transport.EchoHandler = echoHandler
	ret.Transport.Peers = NewPeers()
	ret.Transport.Peers.AddPeers(MakePeerFromServer(server))

	if logger == nil {
		logger = NewLogger(server.ID().String(), log.WARN)
	}
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

// Start starts the raft cluster.
// To work this must be call on a raft pointer which has at least 3 peers.
// Peers can be added from other node but the unstarted cluster must have at least 3 nodes.
func (r *Raft) Start(join bool) error {
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

	// applied, err := r.Storage.LastIndex()
	// if err != nil {
	// 	return err
	// } else if applied != 0 {
	// 	applied = applied - 1
	// }

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
		Applied:         r.applied,
	}

	r.Ticker = time.NewTicker(time.Millisecond * 50)
	r.done = make(chan struct{})

	// If the cluster exist with other nodes
	peers := []raft.Peer{}
	if !join {
		fmt.Println("FRESH !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		peers = r.Transport.Peers.ToRaftPeers()
	} else {
		fmt.Println("JOIN !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}

	fmt.Println("new node has peers", peers)
	r.Node = raft.StartNode(c, peers)
	go r.raftLoop()

	err := r.Transport.HeadToAll(StartNodes, 0)
	if err != nil {
		r.Node.Stop()
		return err
	}

	return nil
}

// AddPeer adds a peer to the cluster. This save the given peer and push
// it to every known peer.
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
	if err != nil {
		return err
	}

	// If the local node is not started or the id is the local node
	if r.Node == nil || peer.ID == r.ID.Uint64() {
		return nil
	}

	return r.join(peer.ID)
}

func (r *Raft) join(peerID uint64) error {
	cc := raftpb.ConfChange{
		Type:   raftpb.ConfChangeAddNode,
		NodeID: peerID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := r.Node.ProposeConfChange(ctx, cc)
	if err != nil {
		return err
	}

	// var entries []raftpb.Entry
	// var first, last uint64
	// first, err = r.Storage.FirstIndex()
	// if err != nil {
	// 	return err
	// }
	// last, err = r.Storage.LastIndex()
	// if err != nil {
	// 	return err
	// }
	// entries, err = r.Storage.Entries(first, last, math.MaxUint64)
	// if err != nil {
	// 	return err
	// }
	// fmt.Println("entries", entries, first, last)

	// hs, _, err := r.Storage.InitialState()
	// if err != nil {
	// 	return err
	// }

	// var hsAsBytes []byte
	// hsAsBytes, err = hs.Marshal()
	// if err != nil {
	// 	return err
	// }

	// joinObj := &struct {
	// 	HS      []byte
	// 	Entries [][]byte
	// 	Applied uint64
	// }{
	// 	HS:      hsAsBytes,
	// 	Entries: make([][]byte, len(entries)),
	// 	Applied: r.Node.Status().Applied,
	// }

	// for _, entry := range entries {
	// 	entryAsBytes, err := entry.Marshal()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	joinObj.Entries = append(joinObj.Entries, entryAsBytes)
	// }

	// var joinObjAsBytes []byte
	// joinObjAsBytes, err = json.Marshal(joinObj)
	// if err != nil {
	// 	return err
	// }

	err = r.Transport.Head(peerID, JoinCluster, 0)
	if err != nil {
		return err
	}

	return nil
}

func (r *Raft) RemovePeer(peerID uint64) error {
	if r.Transport.Peers.Len() <= 3 {
		return ErrNotEnoughNodesForRemove
	}
	if r.Node == nil {
		return ErrRaftNodeNotLoaded
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	err := r.Node.ProposeConfChange(ctx, raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: peerID,
	})
	if err != nil {
		return err
	}

	// err = r.Transport.PostJSONToAll(RMNode, []byte(fmt.Sprintf("%d", peerID)), DefaultRequestTimeOut)
	// if err != nil {
	// 	return err
	// }

	// r.Transport.Peers.RMPeer(peerID)
	// r.rmNode(peerID)

	return nil
}

// LocalPeer returns a Peer pointer of the local raft node
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
					r.confState = r.Node.ApplyConfChange(cc)
					if cc.Type == raftpb.ConfChangeRemoveNode {
						r.rmNode(cc.NodeID)
					}
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
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		r.Messages <- entry.Data
	}
}

// StopRaft stops all raft elements
func (r *Raft) StopRaft() {
	r.Node.Stop()
	r.Ticker.Stop()
	r.done <- struct{}{}
	r.Node = nil
}

// Close stops raft components and deregister the service
func (h *Handler) Close() {
	h.Raft.StopRaft()
	close(h.Raft.Messages)
	h.Handler.Close()
	h.Server.DeregisterService(h.Name())
}

func (r *Raft) addNode(peers ...*Peer) error {
	// Clean from already known peers
	newPeers := []*Peer{}
	for _, newPeer := range peers {
		found := false
		for _, savedPeer := range r.Transport.Peers.GetPeers() {
			if savedPeer.ID == newPeer.ID {
				found = true
				break
			}
		}

		if !found {
			newPeers = append(newPeers, newPeer)
		}
	}

	r.Transport.Peers.AddPeers(peers...)

	if r.Node == nil {
		return nil
	}

	// for _, peer := range newPeers {
	// 	if peer.ID == r.ID.Uint64() {
	// 		continue
	// 	}

	// 	cc := raftpb.ConfChange{
	// 		Type:   raftpb.ConfChangeAddNode,
	// 		NodeID: peer.ID,
	// 	}

	// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// 	defer cancel()
	// 	// r.Node.ApplyConfChange(cc)
	// 	return r.Node.ProposeConfChange(ctx, cc)
	// }
	return nil
}

func (r *Raft) rmNode(peerID uint64) error {
	r.Transport.Peers.RMPeer(peerID)

	// if r.Node == nil {
	// 	return nil
	// }

	// cc := raftpb.ConfChange{
	// 	Type:   raftpb.ConfChangeRemoveNode,
	// 	NodeID: peerID,
	// }

	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// defer cancel()
	// r.Node.ApplyConfChange(cc)
	// err := r.Node.ProposeConfChange(ctx, cc)
	// if err != nil {
	// 	return err
	// }

	if peerID == r.ID.Uint64() {
		r.stopNode()
	}
	return nil
}

func (r *Raft) stopNode() {
	r.Node.Stop()
	r.Node = nil
	r.Ticker.Stop()
	r.Logger = nil
	r.done <- struct{}{}
	close(r.done)
	r.done = nil
}

func (r *Raft) GetConfState() (ret raftpb.ConfState) {
	return *r.confState
}
