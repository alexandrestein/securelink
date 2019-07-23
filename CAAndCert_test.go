package securelink_test

import (
	"crypto/tls"
	"net"
	"reflect"
	"testing"

	"github.com/alexandre/securelink"
)

func TestNewCA(t *testing.T) {
	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
		Error  bool
	}{
		{"Curve 25519", securelink.KeyTypeEd25519, securelink.KeyLengthEd25519, false, false},

		{"EC 256", securelink.KeyTypeEc, securelink.KeyLengthEc256, false, false},
		{"EC 384", securelink.KeyTypeEc, securelink.KeyLengthEc384, false, false},
		{"EC 521", securelink.KeyTypeEc, securelink.KeyLengthEc521, false, false},

		{"RSA 2048", securelink.KeyTypeRSA, securelink.KeyLengthRsa2048, false, false},
		{"RSA 3072", securelink.KeyTypeRSA, securelink.KeyLengthRsa3072, true, false},
		{"RSA 4096", securelink.KeyTypeRSA, securelink.KeyLengthRsa4096, true, false},
		{"RSA 8192", securelink.KeyTypeRSA, securelink.KeyLengthRsa8192, true, false},

		{"not valid", securelink.KeyTypeRSA, securelink.KeyLengthEc256, false, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if test.Long && testing.Short() {
				t.SkipNow()
			}

			conf := securelink.NewDefaultCertificationConfig()
			conf.KeyType = test.Type
			conf.KeyLength = test.Length
			conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

			ca, err := securelink.NewCA(conf, "ca")
			if err != nil {
				if test.Error {
					return
				}
				t.Fatal(err)
			}

			listen(t, ca)

			runClient(t, ca)
		})
	}
}

func listen(t *testing.T, ca *securelink.Certificate) {
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{ca.GetTLSCertificate()},
		ClientCAs:    ca.GetCertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	listener, err := tls.Listen("tcp", ":1323", serverTLSConfig)
	if err != nil {
		t.Fatal(err)
	}

	go func(listener net.Listener) {
		defer listener.Close()
		netConn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}

		tlsConn := netConn.(*tls.Conn)
		err = tlsConn.Handshake()
		if err != nil {
			t.Fatal(err)
		}

		readBuffer := make([]byte, 1000)
		var n, n2 int
		n, err = netConn.Read(readBuffer)
		if err != nil {
			t.Fatal(err)
		}

		readBuffer = readBuffer[:n]

		n2, err = netConn.Write(readBuffer)
		if err != nil {
			t.Fatal(err)
		}

		if n != n2 {
			t.Fatal("the read and write length are not equal", n, n2)
		}
	}(listener)
}

func runClient(t *testing.T, ca *securelink.Certificate) {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert, err := ca.NewCert(conf, "cli")
	if err != nil {
		t.Fatal(err)
	}

	clientTLSConfig := securelink.GetBaseTLSConfig("cli", cert)
	clientTLSConfig.ServerName = "ca"

	var tlsConn *tls.Conn
	tlsConn, err = tls.Dial("tcp", "localhost:1323", clientTLSConfig)
	if err != nil {
		t.Fatal(err)
	}

	err = tlsConn.Handshake()
	if err != nil {
		t.Fatal(err)
	}

	var n, n2 int
	n, err = tlsConn.Write([]byte("HELLO"))
	if err != nil {
		t.Fatal(err)
	}

	readBuff := make([]byte, n)
	n2, err = tlsConn.Read(readBuff)
	if err != nil {
		t.Fatal(err)
	}

	if n != n2 {
		t.Fatal("the returned content is not the same length as the sent one", n2, n)
	}

	if string(readBuff) != "HELLO" {
		t.Fatal("the returned content is not the same as the sent one", string(readBuff), "HELLO")
	}
}

func TestCertificateMarshaling(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

	ca, _ := securelink.NewCA(conf, "ca")

	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
	}{
		{"Curve 25519", securelink.KeyTypeEd25519, securelink.KeyLengthEd25519, false},

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

			cert, err := ca.NewCert(conf, "node1")
			if err != nil {
				t.Fatal(err)
			}

			asBytes := cert.Marshal()

			cert2, err := securelink.Unmarshal(asBytes)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(cert, cert2) {
				t.Fatalf("certificates are not equal\n%v\n%v", cert, cert2)
			}
		})
	}

}
