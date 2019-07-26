// +build go1.1 go1.2 go1.3 go1.4 go1.5 go1.6 go1.7 go1.8 go1.9 go1.10 go1.11 go1.12

package cluster_test

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alexandrestein/securelink/cluster"
	"github.com/alexandrestein/securelink/common"

	"github.com/alexandrestein/securelink"
)

func TestToken(t *testing.T) {
	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
	}{
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
			conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
			conf.KeyType = test.Type
			conf.KeyLength = test.Length

			ca, err := securelink.NewCA(conf, "srv")
			if err != nil {
				t.Fatal(err)
			}

			tlsConfig := securelink.GetBaseTLSConfig("srv", ca)
			var s *securelink.Server
			s, err = securelink.NewServer(1246, tlsConfig, ca, nil)
			if err != nil {
				t.Fatal(err)
			}
			node := cluster.StartNode(s)
			defer node.Close()

			var token string
			token, err = node.GetToken()
			if err != nil {
				t.Fatal(err)
			}

			var addr *common.Addr
			var certFromToken *securelink.Certificate
			addr, certFromToken, err = cluster.ReadToken(token)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(addr, s.Addr()) {
				t.Fatalf("the addresses are not equal: %v %v", addr, s.Addr())
			}
			var conn net.Conn
			conn, err = securelink.NewServiceConnector(addr.String(), "srv", certFromToken, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			tlsConn := conn.(*securelink.TransportConn)
			err = tlsConn.Handshake()
			if err != nil {
				t.Fatal(err)
			}

			err = s.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}