package rafthandler_test

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/rafthandler"
)

var (
	peers = []*rafthandler.Peer{}
)

func TestRaft(t *testing.T) {
	servers, handlers := startNServer(t, 5)
	defer close(servers)

	time.Sleep(time.Second * 5)

	fmt.Println("status", handlers[0].Raft.Node.Status())
	fmt.Println("status", handlers[0].Raft.Node.Status().Progress)

	fmt.Println(servers)
	fmt.Println(handlers)
}

func startNServer(t *testing.T, nb int) ([]*securelink.Server, []*rafthandler.Handler) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	peers := rafthandler.NewPeers()

	servers := make([]*securelink.Server, nb)
	handlers := make([]*rafthandler.Handler, nb)
	for i := 0; i < nb; i++ {
		servers[i], handlers[i] = buildHandler(t, ca, i)

		peers.AddPeers(rafthandler.MakePeerFromServer(servers[i]))
	}

	for i := 0; i < nb; i++ {
		startRaft(t, handlers[i], peers.GetPeers())
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

	raftHandler, err := rafthandler.New(s.Addr(), rafthandler.HostPrefix, s, nil)
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

func close(servers []*securelink.Server) {
	for _, s := range servers {
		s.Close()
	}
}
