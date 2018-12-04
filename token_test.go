package securelink_test

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alexandrestein/common"
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

			conf := securelink.NewDefaultCertificationConfig(nil)
			conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

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
			defer s.Close()

			var token string
			token, err = s.GetToken()
			if err != nil {
				t.Fatal(err)
			}

			var addr *common.Addr
			var certFromToken *securelink.Certificate
			addr, certFromToken, err = securelink.ReadToken(token)
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
