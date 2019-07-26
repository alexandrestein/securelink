package cluster

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/common"
)

type Node struct {
	*securelink.Server
	*InternalNode

	Master *big.Int

	Nodes []*InternalNode
}

type InternalNode struct {
	ID       *big.Int
	Addr     *common.Addr
	IsMaster bool
}

// StartNode start the node services and returns the Node pointer
func StartNode(server *securelink.Server) *Node {
	n := &Node{
		Server: server,
		Master: big.NewInt(0),
		InternalNode: &InternalNode{
			ID:       server.Certificate.ID(),
			IsMaster: false,
			Addr:     server.AddrStruct,
		},
		Nodes: []*InternalNode{},
	}

	n.initHandlers()

	return n
}

func (n *Node) Join(token string) error {
	addr, tmpCertificate, err := ReadToken(token)
	if err != nil {
		return err
	}

	cli := securelink.NewHTTPSConnector(tmpCertificate.CACert.SerialNumber.String(), tmpCertificate)
	var resp *http.Response
	resp, err = cli.Post(buildNodeBaseURL(addr, "join"), "application/json")
	if err != nil {
		return err
	}

	fmt.Println(resp.StatusCode, buildNodeBaseURL(addr, "join"))

	return nil
}

func (n *Node) BootStrap() error {
	if n.ID.Int64() != 0 {
		return fmt.Errorf("the node has a master")
	}
	if n.IsMaster == true {
		return fmt.Errorf("the node is the master but not defined by mater ID")
	}

	n.IsMaster = true
	n.Master = n.Certificate.ID()

	return nil
}

func (n *Node) SetMaster(id *big.Int) error {
	return nil
}

func buildNodeBaseURL(addr *common.Addr, path string) string {
	return fmt.Sprintf("https://%s/_cluster/%s", addr.String(), path)
}