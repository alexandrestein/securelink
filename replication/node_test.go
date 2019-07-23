package replication_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/gotinydb/replication"
	"github.com/alexandrestein/securelink"
)

var (
	ca *securelink.Certificate
)

func init() {
	ca, _ = securelink.NewCA(securelink.KeyTypeEc, securelink.KeyLengthEc256, time.Hour, securelink.GetCertTemplate(nil, nil), "ca")
}

func TestOneNode(t *testing.T) {
	cert, _ := ca.NewCert(securelink.KeyTypeEc, securelink.KeyLengthEc256, time.Hour, securelink.GetCertTemplate(nil, nil), "node")

	// var n *replication.Node
	n, err := replication.NewNode(1255, cert)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("n", n)
}

func TestThreeNodes(t *testing.T) {
	cert1, _ := ca.NewCert(securelink.KeyTypeEc, securelink.KeyLengthEc256, time.Hour, securelink.GetCertTemplate(nil, nil))
	cert2, _ := ca.NewCert(securelink.KeyTypeEc, securelink.KeyLengthEc256, time.Hour, securelink.GetCertTemplate(nil, nil))
	// cert3, _ := ca.NewCert(securelink.KeyTypeEc, securelink.KeyLengthEc256, time.Hour, securelink.GetCertTemplate(nil, nil))

	node1, err := replication.NewNode(31001, cert1)
	if err != nil {
		t.Fatal(err)
	}

	var node2 *replication.Node
	node2, err = replication.NewNode(31002, cert2)
	if err != nil {
		t.Fatal(err)
	}

	nbReached := 0
	nbReached, err = node1.Join([]string{
		node2.Server.AddrStruct.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("nbReached", nbReached)

	// var node3 *replication.Node
	// node3, err = replication.NewNode(addrs3, rs3, dbPath3+"/raftStore", cert3, false)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// node3.GetID().String()
	// f = node1.AddVoter(raft.ServerID(node3.GetID().String()), raft.ServerAddress(node3.Addr.String()))
	// if err := f.Error(); err != nil {
	// 	t.Fatal(err)
	// }

	// fmt.Println("node2 conf", node2.Raft.GetConfiguration().Configuration().Servers)

	// time.Sleep(time.Second * 25)

	// fmt.Println("node2 conf", node2.Raft.GetConfiguration().Configuration().Servers)
}
