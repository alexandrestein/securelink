package cluster_test

import (
	"testing"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/cluster"
)

func TestMain(t *testing.T) {
	certConf := securelink.NewDefaultCertificationConfigWithDefaultTemplate("node1")
	firstNodeCert, _ := securelink.NewCA(certConf, "node1")

	firstNodeTLSConf := securelink.GetBaseTLSConfig("node1", firstNodeCert)
	firstServer, err := securelink.NewServer(5001, firstNodeTLSConf, firstNodeCert, nil)
	if err != nil {
		t.Error(err)
		return
	}

	node1 := cluster.StartNode(firstServer)
	defer node1.Close()

	err = node1.BootStrap()
	if err != nil {
		t.Error(err)
		return
	}

	secondNodeCert, _ := securelink.NewCA(securelink.NewDefaultCertificationConfigWithDefaultTemplate("node2"))
	secondNodeTLSConf := securelink.GetBaseTLSConfig("node2", secondNodeCert)
	var secondServer *securelink.Server
	secondServer, err = securelink.NewServer(5002, secondNodeTLSConf, secondNodeCert, nil)
	if err != nil {
		t.Error(err)
		return
	}

	node2 := cluster.StartNode(secondServer)
	defer node2.Close()

	token, _ := node1.GetToken()

	err = node2.Join(token)
	if err != nil {
		t.Error(err)
		return
	}
}
