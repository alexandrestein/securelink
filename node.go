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
		fmt.Println("can't get update", err)
		return
	}

	var content []byte
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("can't read update", err)
		return
	}

	mp := new(clusterMap)
	err = json.Unmarshal(content, mp)
	if err != nil {
		fmt.Println("can't unmarshal update", err)
		return
	}

	n.lock.Lock()
	n.clusterMap = mp
	n.lock.Unlock()
}

func (n *Node) getMaster() *Peer {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if len(n.clusterMap.Peers) == 0 {
		return n.LocalConfig
	}

	master := n.clusterMap.Peers[0]
	for _, peer := range n.clusterMap.Peers {
		if master.Priority < peer.Priority {
			master = peer
		}
	}

	return master
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

	n.clusterMap.Peers = append(n.clusterMap.Peers, peer)
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

func (n *Node) buildURL(peer *Peer, path string) string {
	ret := fmt.Sprintf("http://%s%s", peer.Addr.String(), path)
	return ret
}

func (n *Node) pingPeers() {
	master := n.getMaster()

	n.lock.RLock()

	mp := n.clusterMap
	pStruct := &ping{
		Time:   mp.Update,
		Master: master.ID,
	}
	asJOSN, err := json.Marshal(pStruct)
	if err != nil {
		return
	}

	for _, peer := range n.clusterMap.Peers {
		if peer.ID.Uint64() == n.LocalConfig.ID.Uint64() {
			continue
		}

		buff := bytes.NewBuffer(asJOSN)

		n.Server.Logger.Debugf("pinging %s at %s from %s at %s", peer.ID.String(), peer.Addr.String(), n.LocalConfig.ID.String(), n.LocalConfig.Addr.String())
		n.Server.Logger.Tracef("pinging info from %s: %s", n.LocalConfig.ID.String(), string(asJOSN))

		cli := n.GetClient()
		resp, err := cli.Post(n.buildURL(peer, "/ping"), "application/json", buff)
		if err != nil {
			n.Server.Logger.Debugln("*Node.pingPeers post: ", err)
			peer.Priority = 0
			continue
		}

		body, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			n.Server.Logger.Debugln("*Node.pingPeers read: ", readErr)
			continue
		}

		prStruct := new(ping)
		jsonErr := json.Unmarshal(body, prStruct)
		if jsonErr != nil {
			n.Server.Logger.Debugln("*Node.pingPeers unmarshal: ", readErr)
			continue
		}

		// Other node has an other config.
		// A call is made to get the new status from master
		if prStruct.Time.After(mp.Update) {
			fmt.Println("remote node " + peer.ID.String() + " is in the future check")
			go n.getUpdate(prStruct.Master)
		}
	}
	n.lock.RUnlock()
}
func (n *Node) checkPeers() {
	ticker := time.NewTicker(time.Second * 2)
	for {
		select {
		case <-ticker.C:
			go n.pingPeers()
		case <-n.Server.Ctx().Done():
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
