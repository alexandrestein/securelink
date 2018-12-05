package rafthandler

import (
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/hashicorp/raft"
)

const (
	// HostPrefix defines the prefix used to define the service.
	// If the node id is XXX the host name provided during handshake will be
	// <HostPrefix>.XXX.
	HostPrefix = "raft"
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

		Server    *securelink.Server
		Raft      *raft.Raft
		Storage   *Storage
		Transport *Transport
	}

	Storage interface {
		raft.LogStore
		raft.StableStore
		raft.SnapshotStore
	}

	Transport struct {
		*securelink.Server
		*securelink.BaseListener
	}
)

func New(addr net.Addr, name string, server *securelink.Server) (*Handler, error) {
	ret := new(Handler)
	ret.Handler = securelink.NewHandler(name, matchRaftPrefixRegexpFunc, ret.Handle)

	ret.Server = server

	return ret, nil
}

func (h *Handler) Start(id string, raftStore Storage, bootstrap bool, servers []raft.Server, notifyChan chan bool) (err error) {
	// n.RaftChan = make(chan bool, 10)
	raftConfig := GetRaftConfig(id, notifyChan)

	err = raft.ValidateConfig(raftConfig)
	if err != nil {
		return err
	}

	h.Transport = &Transport{
		h.Server, securelink.NewBaseListener(h.Server.Addr()),
	}

	tr := raft.NewNetworkTransport(h.Transport, 10, time.Second*2, nil)
	// conn, err := h.Transport.Dial(raft.ServerAddress("ok"), time.Second)
	// fmt.Println("conn, err", conn, err)

	if bootstrap {
		serversConf := raft.Configuration{}

		if servers == nil {
			raftConfig.StartAsLeader = true
			serversConf.Servers = []raft.Server{h.getRaftServer(id)}
		} else {
			serversConf.Servers = servers
		}
		// if hosts == nil || len(hosts) <= 0 {
		// } else {
		// 	servers.Servers = hosts
		// }

		// fmt.Println("init raft", servers.Servers)
		// fmt.Println("hosts", hosts)

		err = raft.BootstrapCluster(raftConfig, raftStore, raftStore, raftStore, tr, serversConf)
		if err != nil {
			return err
		}
	}

	h.Raft, err = raft.NewRaft(raftConfig, nil, raftStore, raftStore, raftStore, tr)
	if err != nil {
		return err
	}

	return err
}

func (h *Handler) Handle(conn net.Conn) error {
	h.Transport.AcceptChan <- conn
	return nil
}

func (h *Handler) getRaftServer(id string) raft.Server {
	return raft.Server{
		Suffrage: raft.Voter,
		ID:       raft.ServerID(id),
		Address:  raft.ServerAddress(h.Server.Addr().String()),
	}
}

func GetRaftConfig(id string, notifyChan chan bool) *raft.Config {
	return &raft.Config{
		ProtocolVersion:    raft.ProtocolVersionMax,
		HeartbeatTimeout:   time.Second * 1,
		ElectionTimeout:    time.Second * 2,
		CommitTimeout:      time.Millisecond * 500,
		MaxAppendEntries:   500,
		ShutdownOnRemove:   true,
		TrailingLogs:       1000,
		SnapshotInterval:   time.Second * 5,
		SnapshotThreshold:  100,
		LeaderLeaseTimeout: time.Millisecond * 1000,
		StartAsLeader:      false,
		LocalID:            raft.ServerID(id),
		NotifyCh:           notifyChan,
	}
}

func BuildRaftServer(id string, addr string, surface raft.ServerSuffrage) raft.Server {
	return raft.Server{
		Suffrage: surface,
		ID:       raft.ServerID(id),
		Address:  raft.ServerAddress(addr),
	}
}
