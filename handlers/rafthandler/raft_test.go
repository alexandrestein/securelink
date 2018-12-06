package rafthandler_test

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/rafthandler"
	"github.com/hashicorp/raft"
)

func TestRaft(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert1, _ := ca.NewCert(conf, "1")
	s1, service1 := startNode(t, cert1, 3121, nil)
	defer s1.Close()

	fmt.Println("s", s1)
	fmt.Println("ss", service1)

	// conf = securelink.NewDefaultCertificationConfig(nil)
	// conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	// cert2, _ := ca.NewCert(conf, "2")
	// // s2, service2 := startNode(t, cert2, 3122, nil)
	// s2, service2 := startNode(t, cert2, 3122, []raft.Server{
	// 	rafthandler.BuildRaftServer(cert1.ID().String(), ":3121", raft.Voter),
	// })
	// defer s2.Close()

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

	time.Sleep(time.Second * 15)

	// fmt.Println("service1.Raft.GetConfiguration().Configuration().Servers", service1.Raft.GetConfiguration().Configuration().Servers)
	// fmt.Println("service3.Raft.GetConfiguration().Configuration().Servers", service3.Raft.GetConfiguration().Configuration().Servers)
}

func startNode(t *testing.T, cert *securelink.Certificate, port uint16, servers []raft.Server) (*securelink.Server, *rafthandler.Handler) {
	getNameFn := func(s string) string {
		return securelink.GetID(s, cert)
	}

	tlsConfig := securelink.GetBaseTLSConfig(cert.ID().String(), cert)
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	s, err := securelink.NewServer(port, tlsConfig, cert, getNameFn)
	if err != nil {
		t.Fatal(err)
	}

	addr := &addr{port}
	raftService, err := rafthandler.New(addr, rafthandler.HostPrefix, s)
	if err != nil {
		t.Fatal(err)
	}

	// notifyChan := make(chan bool)
	// go func() {
	// 	for {
	// 		bool := <-notifyChan
	// 		fmt.Println("notify", port, bool)
	// 	}
	// }()

	// tt := newTT()
	err = raftService.Raft.Start(true, nil)
	if err != nil {
		t.Fatal(err)
	}

	s.RegisterService(raftService)

	return s, raftService
}

type (
	addr struct {
		port uint16
	}

	testTransport struct {
		*raft.InmemStore
		*raft.DiscardSnapshotStore
	}
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
