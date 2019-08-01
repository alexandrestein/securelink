package securelink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/alexandrestein/securelink/common"
	"github.com/labstack/echo"
)

var (
	NodeServiceName = "node securelink internal service"
)

type (
	Node struct {
		Server *Server

		EchoServer *echo.Echo

		clusterMap *clusterMap

		lock        *sync.RWMutex
		LocalConfig *Peer
	}

	Peer struct {
		ID       *big.Int
		Addr     *common.Addr
		Priority float32
		// Path     *big.Int  `json:"-"`
		Update time.Time `json:",omitempty"`
	}

	clusterMap struct {
		Peers  []*Peer
		Update time.Time
	}
)

func NewNode(s *Server, nodeConf *Peer) (*Node, error) {
	config := new(Peer)
	*config = *nodeConf
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
	}

	if config.Priority > 1 || config.Priority < 0 {
		return nil, fmt.Errorf("config priority must be between 0 and 1")
	}

	config.ID = s.Certificate.ID()
	config.Addr = s.AddrStruct

	ln, err := s.NewListener(NodeServiceName)
	if err != nil {
		return nil, err
	}

	n := &Node{
		Server: s,

		EchoServer: NewHTTPEchoServer(ln),

		lock: &sync.RWMutex{},

		clusterMap: &clusterMap{
			Peers:  make([]*Peer, 0),
			Update: time.Now(),
		},

		LocalConfig: config,
	}

	n.clusterMap.Peers = append(n.clusterMap.Peers, config)

	// join saves the list of nodes. The master contact the node to send it the map
	n.EchoServer.POST("/join", n.joinHandler)
	n.EchoServer.POST("/ping", n.pingHandler)
	n.EchoServer.GET("/update", n.updateHandler)
	n.EchoServer.POST("/failure", n.failureHandler)

	go n.EchoServer.StartServer(&http.Server{})

	go n.checkPeers()

	n.Server.Logger.Infof("server started as node with priority of: %f", config.Priority)

	return n, nil
}

func (n *Node) getUpdate(masterID *big.Int) {
	var masterPeer *Peer
	if masterID == nil {
		masterPeer = n.getMaster()
	} else {
		masterPeer = n.getPeer(masterID)
	}

	cli := n.GetClient()
	resp, err := cli.Get(n.buildURL(masterPeer, "/update"))
	if err != nil {
		n.Server.Logger.Warningf("can't get update: %s", err.Error())
		return
	}

	var content []byte
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		n.Server.Logger.Warningf("can't read update: %s", err.Error())
		return
	}

	mp := new(clusterMap)
	err = json.Unmarshal(content, mp)
	if err != nil {
		n.Server.Logger.Warningf("can't unmarshal update: %s", err.Error())
		return
	}

	n.lock.Lock()
	sortPeersBy(sortPeersByPriority).Sort(mp.Peers)
	n.clusterMap = mp
	n.lock.Unlock()

	// niceMap, _ := json.MarshalIndent(mp, "", "	")
	// n.Server.Logger.Infof("cluster map updated: %s", string(niceMap))
	n.Server.Logger.Infof("cluster map updated to: %s", mp.Update.String())
}

// AddPeer can only be run by the master
func (n *Node) AddPeer(peer *Peer) error {
	if master := n.getMaster(); master.ID.Uint64() != n.LocalConfig.ID.Uint64() {
		return fmt.Errorf("the node %q is not master. master is %q", n.LocalConfig.ID.String(), master.ID.String())
	}

	if n.LocalConfig.Priority < peer.Priority {
		return fmt.Errorf("the peer has a bigger priority %s:%f than the master %s:%f", peer.ID.String(), peer.Priority, n.LocalConfig.ID.String(), n.LocalConfig.Priority)
	}

	n.lock.Lock()
	for _, existingPeer := range n.clusterMap.Peers {
		if existingPeer.Priority == peer.Priority {
			n.lock.Unlock()
			return fmt.Errorf("the node %q is has the same priority %f", existingPeer.ID.String(), existingPeer.Priority)
		}
	}

	// Append new element and reorder the list accordingly
	n.clusterMap.Peers = append(n.clusterMap.Peers, peer)
	sortPeersBy(sortPeersByPriority).Sort(n.clusterMap.Peers)

	asJON, err := json.Marshal(n.clusterMap)
	n.clusterMap.Update = time.Now().Truncate(time.Millisecond)
	n.lock.Unlock()
	if err != nil {
		return err
	}

	n.Server.Logger.Infof("added peer %s at %s", peer.ID.String(), peer.Addr.String())

	buff := bytes.NewBuffer(asJON)

	cli := n.GetClient()
	_, err = cli.Post(n.buildURL(peer, "/join"), "application/json", buff)
	if err != nil {
		return err
	}

	go n.pingPeers()

	return nil
}

// RemovePeer removes the given peer from the cluster
func (n *Node) RemovePeer(peerID *big.Int) error {
	if !n.IsMaster() {
		return fmt.Errorf("not master")
	}

	// Lock the node config
	n.lock.Lock()

	// Found the peer from the cluster list
	for i, existingPeer := range n.clusterMap.Peers {
		if existingPeer.ID.Uint64() == peerID.Uint64() {
			// Remove the failed peer from the list
			copy(n.clusterMap.Peers[i:], n.clusterMap.Peers[i+1:])
			n.clusterMap.Peers[len(n.clusterMap.Peers)-1] = nil
			n.clusterMap.Peers = n.clusterMap.Peers[:len(n.clusterMap.Peers)-1]

			// Stop the list evaluation
			break
		}
	}

	// Set the new slucter map version
	n.clusterMap.Update = time.Now().Truncate(time.Millisecond)

	// Lock unlock the node configuration
	n.lock.Unlock()

	// breadcast the new status
	go n.pingPeers()

	return nil
}

// TogglePeer change the sign of the priority to desable or enable the peer
func (n *Node) TogglePeer(peerID *big.Int) error {
	if !n.IsMaster() {
		return fmt.Errorf("not master")
	}

	// Lock the node config
	n.lock.Lock()

	// Found the peer from the cluster list
	for _, existingPeer := range n.clusterMap.Peers {
		if existingPeer.ID.Uint64() == peerID.Uint64() {
			// change the sign
			existingPeer.Priority = -existingPeer.Priority

			// Stop the list evaluation
			break
		}
	}

	// Set the new cluster map version
	n.clusterMap.Update = time.Now().Truncate(time.Millisecond)

	// Reorder the peers list
	sortPeersBy(sortPeersByPriority).Sort(n.clusterMap.Peers)

	// Lock unlock the node configuration
	n.lock.Unlock()

	// breadcast the new status
	go n.pingPeers()

	return nil
}

func (n *Node) ChangePeerPriority(peerID *big.Int, newPriority float32) error {
	if !n.IsMaster() {
		return fmt.Errorf("not master")
	}

	// Lock the node config
	n.lock.Lock()

	// Found the peer from the cluster list
	for _, existingPeer := range n.clusterMap.Peers {
		if existingPeer.ID.Uint64() == peerID.Uint64() {
			// change the sign
			existingPeer.Priority = newPriority

			// Stop the list evaluation
			break
		}
	}

	// Set the new slucter map version
	n.clusterMap.Update = time.Now().Truncate(time.Millisecond)

	// Reorder the peers list
	sortPeersBy(sortPeersByPriority).Sort(n.clusterMap.Peers)

	// Lock unlock the node configuration
	n.lock.Unlock()

	// breadcast the new status
	go n.pingPeers()

	return nil
}

func (n *Node) buildURL(peer *Peer, path string) string {
	if peer == nil {
		return ""
	}
	ret := fmt.Sprintf("http://%s%s", peer.Addr.String(), path)
	return ret
}

func (n *Node) pingPeers() {
	master := n.getMaster()

	n.lock.RLock()
	defer n.lock.RUnlock()

	mp := n.clusterMap
	pStruct := &ping{
		Time:   mp.Update,
		Master: master.ID,
	}
	asJOSN, err := json.Marshal(pStruct)
	if err != nil {
		return
	}

	cli := n.GetClient()
	cli.Timeout = time.Millisecond * 500

	failureFn := func(failedPeer *Peer) {
		masterPeer := n.getMaster()

		peerAsJSON, _ := json.Marshal(failedPeer)
		buff := bytes.NewBuffer(peerAsJSON)

		failOver := false
		if masterPeer.ID.Uint64() == failedPeer.ID.Uint64() {
			n.Server.Logger.Warningf("master %s-%s is down", failedPeer.ID.String(), failedPeer.Addr.String())

			masterPeer = n.getMasterFailover()
			failOver = true
		} else {
			n.Server.Logger.Infof("node %s-%s is down", failedPeer.ID.String(), failedPeer.Addr.String())
		}

		_, err := cli.Post(n.buildURL(masterPeer, "/failure"), "application/json", buff)
		if err != nil {
			master := "master"
			if failOver {
				master = "failover node"
			}
			n.Server.Logger.Warningf("from %s-%s %s is unreachable: %s", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String(), master, err.Error())
			return
		}
	}

	for _, peer := range n.clusterMap.Peers {
		// If local
		if peer.ID.Uint64() == n.LocalConfig.ID.Uint64() {
			continue
		}

		// If disable
		if peer.Priority <= 0 {
			continue
		}

		// Make a copy of the peer because in case of failure the copy is used to
		// prevent race on it
		peerCopy := new(Peer)
		*peerCopy = *peer

		buff := bytes.NewBuffer(asJOSN)

		n.Server.Logger.Debugf("pinging %s:%s", peer.ID.String(), peer.Addr.String())
		n.Server.Logger.Tracef("pinging from %s:%s: %s", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String(), string(asJOSN))

		resp, err := cli.Post(n.buildURL(peer, "/ping"), "application/json", buff)
		if err != nil {
			n.Server.Logger.Errorf("*Node.pingPeers %s-%s: %s", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String(), err.Error())

			go failureFn(peerCopy)
			continue
		}

		body, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			n.Server.Logger.Errorf("*Node.pingPeers node %s-%s read: %s", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String(), readErr.Error())
			go failureFn(peerCopy)
			continue
		}

		prStruct := new(ping)
		jsonErr := json.Unmarshal(body, prStruct)
		if jsonErr != nil {
			n.Server.Logger.Errorf("*Node.pingPeers node %s-%s unmarshal: %s", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String(), readErr.Error())
			go failureFn(peerCopy)
			continue
		}

		// Other node has an other config.
		// A call is made to get the new status from master
		if prStruct.Time.After(mp.Update) {
			n.Server.Logger.Infof("remote node %s:%s is in the future check", peer.ID.String(), peer.Addr.String())
			go n.getUpdate(prStruct.Master)
		}
	}
}

func (n *Node) Fail() {

}

func (n *Node) checkPeers() {
	ticker := time.NewTicker(time.Second * 2)
	for {
		select {
		case <-ticker.C:
			go n.pingPeers()
		case <-n.Server.Ctx().Done():
			n.Server.Logger.Infof("the server context is done: %s", n.Server.ctx.Err().Error())
			n.EchoServer.Close()
			return
		}
	}
}

func (n *Node) GetClient() *http.Client {
	fn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return n.Server.Dial(addr, NodeServiceName, time.Second*5)
	}

	dt := &http.Transport{
		DialContext:           fn,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	cli := &http.Client{
		Transport: dt,
	}

	return cli
}

func (n *Node) IsMaster() bool {
	master := n.getMaster()
	if master.ID.Uint64() == n.LocalConfig.ID.Uint64() {
		return true
	}
	return false
}
