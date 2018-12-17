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
	startNServer(t, 5)
	// servers, handlers := startNServer(t, 5)

	time.Sleep(time.Second * 60)
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
		// time.Sleep(time.Second)

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

	raftHandler, err := rafthandler.New(s.Addr(), rafthandler.HostPrefix, s)
	if err != nil {
		t.Fatal(err)
	}

	s.RegisterService(raftHandler)

	return s, raftHandler
}

// func startServer(t *testing.T, cert *securelink.Certificate, port uint16) (*securelink.Server, *rafthandler.Handler) {
// 	// getNameFn := func(s string) string {
// 	// 	return securelink.GetID(s, cert)
// 	// }

// 	tlsConfig := securelink.GetBaseTLSConfig(cert.ID().String(), cert)
// 	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
// 	s, err := securelink.NewServer(port, tlsConfig, cert, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	addr := &addr{port}
// 	raftService, err := rafthandler.New(addr, rafthandler.HostPrefix, s)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// peers = append(peers, rafthandler.MakePeerFromServer(s))

// 	// notifyChan := make(chan bool)
// 	// go func() {
// 	// 	for {
// 	// 		bool := <-notifyChan
// 	// 		fmt.Println("notify", port, bool)
// 	// 	}
// 	// }()

// 	// tt := newTT()

// 	// err = raftService.Raft.Start()
// 	// if err != nil {
// 	// 	t.Fatal(err)
// 	// }

// 	s.RegisterService(raftService)

// 	return s, raftService
// }

func startRaft(t *testing.T, handler *rafthandler.Handler, peers []*rafthandler.Peer) {
	handler.Transport.Peers.AddPeers(peers...)
	err := handler.Raft.Start()
	if err != nil {
		t.Fatal(err)
	}
}

// type (
// 	addr struct {
// 		port uint16
// 	}

// 	// testTransport struct {
// 	// 	*raft.InmemStore
// 	// 	*raft.DiscardSnapshotStore
// 	// }
// )

// func (a *addr) Network() string {
// 	return "tcp"
// }

// func (a *addr) String() string {
// 	return fmt.Sprintf(":%d", a.port)
// }

// func newTT() *testTransport {
// 	return &testTransport{
// 		raft.NewInmemStore(),
// 		raft.NewDiscardSnapshotStore(),
// 	}
// }
