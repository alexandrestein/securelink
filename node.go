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

		lock        *sync.Mutex
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
	// if !s.Certificate.IsCA {
	// 	return nil, fmt.Errorf("the server must have a CA certificate")
	// }

	config := new(Peer)
	*config = *nodeConf
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
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

		lock: &sync.Mutex{},

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

func (n *Node) getUpdate() {
	masterPeer := n.getMaster()

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
	n.lock.Lock()
	defer n.lock.Unlock()

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

	n.lock.Lock()
	n.clusterMap.Peers = append(n.clusterMap.Peers, peer)
	asJON, err := json.Marshal(n.clusterMap)
	n.clusterMap.Update = time.Now().Truncate(time.Millisecond)
	n.lock.Unlock()
	if err != nil {
		return err
	}

	buff := bytes.NewBuffer(nil)
	buff.Write(asJON)

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
	n.lock.Lock()

	mp := n.clusterMap
	asJOSN, err := json.Marshal(mp.Update)
	if err != nil {
		return
	}

	for _, peer := range n.clusterMap.Peers {
		if peer.ID.Uint64() == n.LocalConfig.ID.Uint64() {
			continue
		}

		buff := bytes.NewBuffer(nil)
		buff.Write(asJOSN)

		cli := n.GetClient()
		resp, err := cli.Post(n.buildURL(peer, "/ping"), "application/json", buff)
		if err != nil {
			fmt.Println("err", err)
			peer.Priority = 0
			continue
		}

		body, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			continue
		}

		t := time.Time{}
		jsonErr := json.Unmarshal(body, &t)
		if jsonErr != nil {
			continue
		}

		// Other node has an other config.
		// A call is made to get the new status from master
		if t.After(mp.Update) {
			fmt.Println("remote node " + peer.ID.String() + " is in the future check")
			go n.getUpdate()
		}
	}
	n.lock.Unlock()
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
