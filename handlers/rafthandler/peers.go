package rafthandler

import (
	"fmt"

	"github.com/alexandrestein/common"
	"github.com/etcd-io/etcd/raft"
)

type (
	Peers struct {
		Peers []*Peer
	}
	Peer struct {
		raft.Peer
		*common.Addr
	}
)

func MakePeer(id uint64, addr *common.Addr) *Peer {
	return &Peer{
		Peer: raft.Peer{
			ID: id,
		},
		Addr: addr,
	}
}

func NewPeers(peers ...*Peer) *Peers {
	p := new(Peers)
	p.Peers = []*Peer{}
	p.AddPeers(peers...)

	return p
}

func (p *Peers) AddPeers(peers ...*Peer) {
	p.Peers = append(p.Peers, peers...)
}

func (p *Peers) ToRaftPeers() []raft.Peer {
	retPeers := make([]raft.Peer, len(p.Peers))
	for i, peer := range p.Peers {
		retPeers[i] = peer.Peer
	}
	return retPeers
}

func (p *Peer) BuildURL(input string) string {
	return fmt.Sprintf("https://%s%s", p.String(), input)
}
