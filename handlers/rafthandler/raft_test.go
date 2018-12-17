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
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert1, _ := ca.NewCert(conf, "1")
	fmt.Printf("s1 %d ID: %x\n", cert1.ID().Uint64(), cert1.ID().Bytes())
	conf = securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert2, _ := ca.NewCert(conf, "2")
	fmt.Printf("s2 %d ID: %x\n", cert2.ID().Uint64(), cert2.ID().Bytes())

	s1, service1 := startServer(t, cert1, 3121)
	defer s1.Close()

	time.Sleep(time.Millisecond * 2000)

	// s2, service2 := startNode(t, cert2, 3122, nil)
	s2, service2 := startServer(t, cert2, 3122)
	defer s2.Close()

	// fmt.Println("s", s1, s2)
	fmt.Println("ss", service1, service2)
	// fmt.Println("sdsdd", cert2.ID().String(), s2.ID().String(), service2.Server.ID().String())

	// time.Sleep(time.Second * 5)

	peers = []*rafthandler.Peer{
		rafthandler.MakePeerFromServer(s1),
		rafthandler.MakePeerFromServer(s2),
	}

	startRaft(t, service1)
	startRaft(t, service2)

	// node2Peer := rafthandler.MakePeerFromServer(s2)
	// err := service1.Raft.AddPeer(node2Peer)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	time.Sleep(time.Second * 15)

	fmt.Println("NNNN", service1.Raft.Node.Status().Progress)
	fmt.Println("NNNN", service2.Raft.Node.Status().Progress)

	// conf = securelink.NewDefaultCertificationConfig(nil)
	// conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	// cert3, _ := ca.NewCert(conf, "3")
	// // s3, service3 := startNode(t, cert3, 3123, nil)
	// s3, service3 := startNode(t, cert3, 3123, []raft.Server{
	// 	rafthandler.BuildRaftServer(cert1.ID().String(), ":3121", raft.Voter),
	// 	rafthandler.BuildRaftServer(cert2.ID().String(), ":3122", raft.Voter),
	// })
	// defer s3.Close()

	// fmt.Println(s1, s2, s3)
	// fmt.Println(service1, service2, service3)

	// service1.Raft.AddVoter(raft.ServerID(s2.TLS.Certificate.ID().String()), raft.ServerAddress("127.0.0.1:3122"), 0, time.Second)
	// service1.Raft.AddVoter(raft.ServerID(s3.TLS.Certificate.ID().String()), raft.ServerAddress("127.0.0.1:3123"), 0, time.Second)

	// time.Sleep(time.Second * 15)

	// fmt.Println("service1.Raft.GetConfiguration().Configuration().Servers", service1.Raft.GetConfiguration().Configuration().Servers)
	// fmt.Println("service3.Raft.GetConfiguration().Configuration().Servers", service3.Raft.GetConfiguration().Configuration().Servers)
}

func startNServer(t *testing.T, nb int) ([]*securelink.Server, []*rafthandler.Handler) {
}

func startServer(t *testing.T, cert *securelink.Certificate, port uint16) (*securelink.Server, *rafthandler.Handler) {
	// getNameFn := func(s string) string {
	// 	return securelink.GetID(s, cert)
	// }

	tlsConfig := securelink.GetBaseTLSConfig(cert.ID().String(), cert)
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	s, err := securelink.NewServer(port, tlsConfig, cert, nil)
	if err != nil {
		t.Fatal(err)
	}

	addr := &addr{port}
	raftService, err := rafthandler.New(addr, rafthandler.HostPrefix, s)
	if err != nil {
		t.Fatal(err)
	}

	// peers = append(peers, rafthandler.MakePeerFromServer(s))

	// notifyChan := make(chan bool)
	// go func() {
	// 	for {
	// 		bool := <-notifyChan
	// 		fmt.Println("notify", port, bool)
	// 	}
	// }()

	// tt := newTT()

	// err = raftService.Raft.Start()
	// if err != nil {
	// 	t.Fatal(err)
	// }

	s.RegisterService(raftService)

	return s, raftService
}

func startRaft(t *testing.T, handler *rafthandler.Handler) {
	handler.Transport.Peers.AddPeers(peers...)
	err := handler.Raft.Start()
	if err != nil {
		t.Fatal(err)
	}
}

type (
	addr struct {
		port uint16
	}

	// testTransport struct {
	// 	*raft.InmemStore
	// 	*raft.DiscardSnapshotStore
	// }
)

func (a *addr) Network() string {
	return "tcp"
}

func (a *addr) String() string {
	return fmt.Sprintf(":%d", a.port)
}

// func newTT() *testTransport {
// 	return &testTransport{
// 		raft.NewInmemStore(),
// 		raft.NewDiscardSnapshotStore(),
// 	}
// }
