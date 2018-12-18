package rafthandler

import (
	"fmt"
	"sync"

	"github.com/alexandrestein/common"
	"github.com/alexandrestein/securelink"
	"github.com/etcd-io/etcd/raft"
)

type (
	Peers struct {
		peers []*Peer
		lock  sync.Locker
	}
	Peer struct {
		raft.Peer
		*common.Addr
	}
)

func NewPeers(peers ...*Peer) *Peers {
	p := new(Peers)
	p.peers = []*Peer{}
	p.lock = new(sync.Mutex)
	p.AddPeers(peers...)

	return p
}

func (p *Peers) AddPeers(peers ...*Peer) {
	p.lock.Lock()

	// Check that the ID is not already present
	peersToSave := []*Peer{}
	for _, givenPeer := range peers {
		for _, savedPeer := range p.peers {
			if givenPeer.ID == savedPeer.ID {
				goto pass
			}
		}
		peersToSave = append(peersToSave, givenPeer)
	pass:
	}

	p.peers = append(p.peers, peersToSave...)
	p.lock.Unlock()
}

func (p *Peers) GetPeers() []*Peer {
	return p.peers
}
func (p *Peers) Len() int {
	return len(p.peers)
}

func (p *Peers) ToRaftPeers() []raft.Peer {
	retPeers := make([]raft.Peer, len(p.peers))
	for i, peer := range p.peers {
		retPeers[i] = peer.Peer
	}
	return retPeers
}

func (p *Peers) Empty() bool {
	if len(p.peers) <= 0 {
		return true
	}
	return false
}

func MakePeer(id uint64, addr *common.Addr) *Peer {
	return &Peer{
		Peer: raft.Peer{
			ID: id,
		},
		Addr: addr,
	}
}

func MakePeerFromServer(s *securelink.Server) *Peer {
	return MakePeer(s.ID().Uint64(), s.AddrStruct)
}

func (p *Peer) BuildURL(input string) string {
	return fmt.Sprintf("https://%s%s", p.Addr.String(), input)
}

func (p *Peer) ToRaftPeer() raft.Peer {
	return p.Peer
}
