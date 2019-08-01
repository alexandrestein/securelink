package securelink_test

import (
	"context"
	"crypto/tls"
	"reflect"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/common"
)

func TestStart3Nodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, servers := initServers(ctx, 3, 0)

	s0 := servers[0]
	defer s0.Close()

	s1 := servers[1]
	defer s1.Close()

	s2 := servers[2]
	defer s2.Close()

	config := &securelink.Peer{}

	config.Priority = 0.9
	n0, _ := securelink.NewNode(s0, config)
	config.Priority = 0.5
	n1, _ := securelink.NewNode(s1, config)
	config.Priority = 0.2
	n2, _ := securelink.NewNode(s2, config)

	if testing.Verbose() {
		t.Log("master is " + n0.Server.Certificate.ID().String())
	}

	time.Sleep(time.Second)

	err := n0.AddPeer(n1.LocalConfig)
	if err != nil {
		t.Error(err)
		return
	}

	// Add a peer which must get an error
	config.Priority = 0.5
	err = n0.AddPeer(config)
	if err == nil {
		t.Errorf("expected an error but had no")
		return
	}

	time.Sleep(time.Second * 5)
	err = n0.AddPeer(n2.LocalConfig)
	if err != nil {
		t.Error(err)
		return
	}
	time.Sleep(time.Second * 30)
}

func TestNodeFaillure5To3Nodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, servers := initServers(ctx, 5, 0)

	s0 := servers[0]
	defer s0.Close()
	s0.Logger.SetLevel(logrus.ErrorLevel)

	s1 := servers[1]
	defer s1.Close()
	s1.Logger.SetLevel(logrus.ErrorLevel)

	s2 := servers[2]
	defer s2.Close()
	s2.Logger.SetLevel(logrus.ErrorLevel)

	s3 := servers[3]
	defer s3.Close()
	s3.Logger.SetLevel(logrus.ErrorLevel)

	s4 := servers[4]
	defer s4.Close()
	s4.Logger.SetLevel(logrus.TraceLevel)

	config := &securelink.Peer{}

	config.Priority = 0.9
	n0, _ := securelink.NewNode(s0, config)
	config.Priority = 0.5
	n1, _ := securelink.NewNode(s1, config)
	config.Priority = 0.2
	n2, _ := securelink.NewNode(s2, config)
	config.Priority = 0.55
	n3, _ := securelink.NewNode(s3, config)
	config.Priority = 0.56
	n4, _ := securelink.NewNode(s4, config)

	if testing.Verbose() {
		t.Log("master is " + n0.Server.Certificate.ID().String())
	}

	time.Sleep(time.Second)

	n0.AddPeer(n1.LocalConfig)
	// time.Sleep(time.Second)
	n0.AddPeer(n2.LocalConfig)
	// time.Sleep(time.Second)
	n0.AddPeer(n3.LocalConfig)
	// time.Sleep(time.Second)
	n0.AddPeer(n4.LocalConfig)
	// time.Sleep(time.Second)

	time.Sleep(time.Second * 3)

	// n3.EchoServer.Close()
	s3.Close()

	time.Sleep(time.Second * 15)

	// n0.EchoServer.Close()
	s0.Close()

	time.Sleep(time.Second * 30)
}

func TestNodeToken(t *testing.T) {
	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
	}{
		// {"Curve 25519", securelink.KeyTypeEd25519, securelink.KeyLengthEd25519, false},

		{"EC 256", securelink.KeyTypeEc, securelink.KeyLengthEc256, false},
		{"EC 384", securelink.KeyTypeEc, securelink.KeyLengthEc384, false},
		{"EC 521", securelink.KeyTypeEc, securelink.KeyLengthEc521, false},

		{"RSA 2048", securelink.KeyTypeRSA, securelink.KeyLengthRsa2048, false},
		{"RSA 3072", securelink.KeyTypeRSA, securelink.KeyLengthRsa3072, true},
		{"RSA 4096", securelink.KeyTypeRSA, securelink.KeyLengthRsa4096, true},
		{"RSA 8192", securelink.KeyTypeRSA, securelink.KeyLengthRsa8192, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if test.Long && testing.Short() {
				t.SkipNow()
			}

			conf := securelink.NewDefaultCertificationConfig()
			conf.KeyType = test.Type
			conf.KeyLength = test.Length
			ca, err := securelink.NewCA(conf, "srv")
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()

			tlsConfig := securelink.GetBaseTLSConfig("srv", ca)
			var s1 *securelink.Server
			s1, err = securelink.NewServer(ctx, 1246, tlsConfig, ca)
			if err != nil {
				t.Fatal(err)
			}
			defer s1.Close()
			config := &securelink.Peer{}

			config.Priority = 0.9
			node, _ := securelink.NewNode(s1, config)

			var token string
			token, err = node.GetToken()
			if err != nil {
				t.Fatal(err)
			}

			var addr *common.Addr
			var certFromToken *securelink.Certificate
			addr, certFromToken, err = securelink.ReadToken(token)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(addr, s1.AddrStruct) {
				t.Fatalf("the addresses are not equal: %v %v", addr, s1.AddrStruct)
			}

			tlsConfigClone := tlsConfig.Clone()
			tlsConfigClone.Certificates = []tls.Certificate{certFromToken.GetTLSCertificate()}
			// var session *quic.Session
			_, err = securelink.DialQuic(tlsConfigClone, addr.String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
