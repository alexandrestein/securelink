package rafthandler

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alexandrestein/common"
	"github.com/alexandrestein/securelink"
	"github.com/etcd-io/etcd/raft"
)

type (
	// Peers defines a slice of Peer. It can be use safely from multiple goroutines
	Peers struct {
		// local *Peer

		peers []*Peer
		lock  *sync.RWMutex
	}
	// Peer defines the remote peer with communication components
	Peer struct {
		raft.Peer
		*common.Addr

		cli         *http.Client
		cliDeadline time.Time

		lock *sync.RWMutex
	}
)

// NewPeers builds a Peers pointer filled-up with the given Peer pointers
func NewPeers(peers ...*Peer) *Peers {
	p := new(Peers)
	// p.local = local
	p.peers = []*Peer{}
	p.lock = new(sync.RWMutex)
	p.AddPeers(peers...)

	return p
}

// AddPeers add a peer to the existing slice
func (p *Peers) AddPeers(peers ...*Peer) {
	// Check that the ID is not already present
	peersToSave := []*Peer{}
	for _, givenPeer := range peers {
		for _, savedPeer := range p.peers {
			if givenPeer.ID == savedPeer.ID {
				goto pass
			}
		}
		givenPeer.lock = new(sync.RWMutex)
		peersToSave = append(peersToSave, givenPeer)
	pass:
	}

	// Try to lock the variable less as possible.
	// Lock only if needed.
	if len(peersToSave) > 0 {
		p.lock.Lock()
		p.peers = append(p.peers, peersToSave...)
		p.lock.Unlock()
	}
}

// RMPeer removes the peers with the same ID
func (p *Peers) RMPeer(peerID uint64) {
	p.lock.Lock()

	toRemove := []int{}
	for i, savedPeer := range p.peers {
		if peerID == savedPeer.ID {
			toRemove = append(toRemove, i)
		}
	}

	for _, i := range toRemove {
		// Remove the peer from the slice
		copy(p.peers[i:], p.peers[i+1:])
		p.peers[len(p.peers)-1] = nil
		p.peers = p.peers[:len(p.peers)-1]
	}

	p.lock.Unlock()
}

// // GetLocalPeer returns the Peer pointer given at the initialization
// func (p *Peers) GetLocalPeer() *Peer {
// 	return p.local
// }

// GetPeers returns a slice of all registered Peer pointers
func (p *Peers) GetPeers() []*Peer {
	return p.peers
}

// Len returns how many peers are registered
func (p *Peers) Len() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return len(p.peers)
}

// ToRaftPeers convert the slice of Peer pointer to
// a slice of raft.Peer variables
func (p *Peers) ToRaftPeers() []raft.Peer {
	retPeers := make([]raft.Peer, len(p.peers))
	for i, peer := range p.peers {
		retPeers[i] = peer.Peer
	}
	return retPeers
}

// Empty return true of no peer are registered
func (p *Peers) Empty() bool {
	if p.Len() <= 0 {
		return true
	}
	return false
}

// MakePeer builds a Peer pointer with the given ID and Addr pointer
func MakePeer(id uint64, addr *common.Addr) *Peer {
	return &Peer{
		Peer: raft.Peer{
			ID: id,
		},
		Addr: addr,
		lock: new(sync.RWMutex),
	}
}

// MakePeerFromServer does same as above but based on the given server
func MakePeerFromServer(s *securelink.Server) *Peer {
	return MakePeer(s.ID().Uint64(), s.AddrStruct)
}

// BuildURL build an, URL for the given peer but based on its IP and never on the domain
func (p *Peer) BuildURL(input string) string {
	return fmt.Sprintf("https://%s%s", p.Addr.String(), input)
}

// ToRaftPeer convert the actual pointer to a raftPeer variable
func (p *Peer) ToRaftPeer() raft.Peer {
	return p.Peer
}
