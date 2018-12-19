package rafthandler_test

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/rafthandler"
	"github.com/etcd-io/etcd/raft"
	"github.com/labstack/gommon/log"
)

func TestRaft(t *testing.T) {
	servers, handlers := startNServer(t, 5)
	defer closeServers(servers)

	time.Sleep(time.Second * 2)

	if handlers[0].Raft.Node.Status().Lead == raft.None {
		t.Fatalf("no leader for server 1")
	} else if handlers[0].Raft.Node.Status().Lead != handlers[1].Raft.Node.Status().Lead {
		t.Fatalf("the leader for server 1 and 2 are not equal %d != %d", handlers[0].Raft.Node.Status().Lead, handlers[1].Raft.Node.Status().Lead)
	}
}

func startNServer(t *testing.T, nb int) ([]*securelink.Server, []*rafthandler.Handler) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	servers := make([]*securelink.Server, nb)
	handlers := make([]*rafthandler.Handler, nb)
	for i := 0; i < nb; i++ {
		servers[i], handlers[i] = buildHandler(t, ca, i)
	}

	for _, h := range handlers {
		err := handlers[0].Raft.AddPeer(rafthandler.MakePeerFromServer(h.Server))
		if err != nil {
			closeServers(servers)
			t.Fatal(err)
		}
	}

	time.Sleep(time.Second)

	err := handlers[0].Raft.Start()
	if err != nil {
		closeServers(servers)
		t.Fatal(err)
	}

	return servers, handlers
}

func buildHandler(t *testing.T, ca *securelink.Certificate, nb int) (*securelink.Server, *rafthandler.Handler) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert, _ := ca.NewCert(conf, fmt.Sprintf("%d", nb))

	basePort := uint16(32600)
	port := basePort + uint16(nb)

	tlsConfig := securelink.GetBaseTLSConfig(cert.ID().String(), cert)
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	s, err := securelink.NewServer(port, tlsConfig, cert, nil)
	if err != nil {
		t.Fatal(err)
	}

	raftHandler, err := rafthandler.New(s.Addr(), rafthandler.HostPrefix, s, rafthandler.NewLogger(cert.ID().String(), log.DEBUG))
	if err != nil {
		t.Fatal(err)
	}

	return s, raftHandler
}

func startRaft(t *testing.T, handler *rafthandler.Handler, peers []*rafthandler.Peer) {
	handler.Transport.Peers.AddPeers(peers...)
	err := handler.Raft.Start()
	if err != nil {
		t.Fatal(err)
	}
}

func closeServers(servers []*securelink.Server) {
	for _, s := range servers {
		s.Close()
	}
}
