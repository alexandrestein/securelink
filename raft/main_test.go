package raft

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/securelink"
)

func initServers(ctx context.Context, n int) (ca *securelink.Certificate, servers []*securelink.Server) {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ = securelink.NewCA(conf, "ca")

	servers = make([]*securelink.Server, n)
	for i := range servers {
		conf = securelink.NewDefaultCertificationConfig()
		conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
		cert, _ := ca.NewCert(conf)
		server, _ := securelink.NewServer(ctx, 3160+uint16(i), securelink.GetBaseTLSConfig(fmt.Sprint(i), cert), cert)

		servers[i] = server
	}

	return
}

func TestNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, servers := initServers(ctx, 3)

	s0 := servers[0]
	defer s0.Close()

	s1 := servers[1]
	defer s1.Close()

	s2 := servers[2]
	defer s2.Close()

	n0, _ := StartNode(s0, []uint64{s1.Certificate.ID().Uint64(), s2.Certificate.ID().Uint64()})
	n1, _ := StartNode(s1, []uint64{s0.Certificate.ID().Uint64(), s2.Certificate.ID().Uint64()})
	n2, _ := StartNode(s2, []uint64{s0.Certificate.ID().Uint64(), s1.Certificate.ID().Uint64()})

	time.Sleep(time.Second)

	err := n0.Campaign(ctx)
	fmt.Println("err", err)

	time.Sleep(time.Second * 60)

	fmt.Println(n0)
	fmt.Println(n1)
	fmt.Println(n2)
}
