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

	Nodes []*InternalNode
}

type InternalNode struct {
	Master   *big.Int
	IsMaster bool
}

// StartNode start the node services and returns the Node pointer
func StartNode(server *securelink.Server) *Node {
	return &Node{
		Server:       server,
		InternalNode: &InternalNode{Master: big.NewInt(0)},
		Nodes: []*InternalNode{
			{
				server.Certificate.ID(),
				false,
			},
		},
	}
}

func (n *Node) Join(token string) error {
	addr, tmpCertificate, err := ReadToken(token)
	if err != nil {
		return err
	}

	cli := securelink.NewHTTPSConnector(tmpCertificate.CACert.SerialNumber.String(), tmpCertificate)
	var resp *http.Response
	resp, err = cli.Get(buildNodeBaseURL(addr, "cluster/join"))
	if err != nil {
		return err
	}

	fmt.Println(resp.StatusCode)

	return nil
}

func (n *Node) BootStrap() error {
	if n.Master.Int64() != 0 {
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
	return fmt.Sprintf("https://%s/%s", addr.String(), path)
}
