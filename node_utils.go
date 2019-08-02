package securelink

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"
	"sort"
	"time"
)

func (n *Node) getMaster() *Peer {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if len(n.clusterMap.Peers) <= 1 {
		return n.LocalConfig
	}

	return n.clusterMap.Peers[0]
}

func (n *Node) getMasterFailover() *Peer {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if len(n.clusterMap.Peers) <= 2 {
		return n.LocalConfig
	}

	return n.clusterMap.Peers[1]
}

func (n *Node) getPeer(id *big.Int) *Peer {
	n.lock.RLock()
	defer n.lock.RUnlock()

	for _, peer := range n.clusterMap.Peers {
		if peer.ID.Uint64() == id.Uint64() {
			return peer
		}
	}

	return nil
}

func (n *Node) checkPeerAlive(peer *Peer) bool {
	if savedPeer := n.getPeer(peer.ID); savedPeer.Priority < 0 {
		return false
	}

	master := n.getMaster()

	cli := n.GetClient()
	cli.Timeout = time.Second

	n.lock.RLock()
	pStruct := &ping{
		Time:   n.clusterMap.Update,
		Master: master.ID,
	}
	n.lock.RUnlock()

	asJSON, _ := json.Marshal(pStruct)
	buff := bytes.NewBuffer(asJSON)

	alive := false
	resp, err := cli.Post(n.buildURL(peer, "/ping"), "application/json", buff)
	if err != nil {
		n.Server.Logger.Infof("*Node.checkPeerAlive node %s-%s did not reply to ping: %s", peer.ID.String(), peer.Addr.String(), err.Error())
		return false
	}

	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		n.Server.Logger.Infof("*Node.checkPeerAlive node %s-%s replied but the response is not OK: %s", peer.ID.String(), peer.Addr.String(), string(msg))
	} else {
		alive = true
	}

	return alive
}

// Sorting !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
type sortPeersBy func(p1, p2 *Peer) bool

func (by sortPeersBy) Sort(peers []*Peer) {
	ps := &peerSorter{
		peers: peers,
		by:    by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type peerSorter struct {
	peers []*Peer
	by    func(p1, p2 *Peer) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *peerSorter) Len() int {
	return len(s.peers)
}

// Swap is part of sort.Interface.
func (s *peerSorter) Swap(i, j int) {
	s.peers[i], s.peers[j] = s.peers[j], s.peers[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *peerSorter) Less(i, j int) bool {
	return s.by(s.peers[i], s.peers[j])
}

func sortPeersByPriority(p1, p2 *Peer) bool {
	return p1.Priority > p2.Priority
}
