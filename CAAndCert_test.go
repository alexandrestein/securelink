package securelink_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/alexandrestein/securelink"
)

func TestNewCA(t *testing.T) {
	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
		Error  bool
	}{
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

			conf := securelink.NewDefaultCertificationConfig(nil)
			conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

			ca, err := securelink.NewCA(conf, "ca")
			if err != nil {
				if test.Error {
					return
				}
				t.Fatal(err)
			}

			errChan := listen(t, ca)

			runClient(t, ca)

			err = <-errChan
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func listen(t *testing.T, ca *securelink.Certificate) (errChan chan error) {
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{ca.GetTLSCertificate()},
		ClientCAs:    ca.GetCertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	listener, err := tls.Listen("tcp", ":1323", serverTLSConfig)
	if err != nil {
		t.Fatal(err)
	}

	errChan = make(chan error)
	go func(listener net.Listener, errChan chan error) {
		defer func() { errChan <- nil }()

		netConn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

		tlsConn := netConn.(*tls.Conn)
		err = tlsConn.Handshake()
		if err != nil {
			errChan <- err
			return
		}

		readBuffer := make([]byte, 1000)
		var n, n2 int
		n, err = netConn.Read(readBuffer)
		if err != nil {
			errChan <- err
			return
		}

		readBuffer = readBuffer[:n]

		n2, err = netConn.Write(readBuffer)
		if err != nil {
			errChan <- err
			return
		}

		if n != n2 {
			errChan <- fmt.Errorf("the read and write length are not equal %d %d", n, n2)
			return
		}
	}(listener, errChan)

	return errChan
}

func runClient(t *testing.T, ca *securelink.Certificate) {
	conf := securelink.NewDefaultCertificationConfig(ca)
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
	conf := securelink.NewDefaultCertificationConfig(nil)
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

	ca, _ := securelink.NewCA(conf, "ca")

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

			cert, err := ca.NewCert(conf, "node1")
			if err != nil {
				t.Fatal(err)
			}

			var asBytes []byte
			asBytes, err = cert.Marshal()
			if err != nil {
				t.Fatal(err)
			}

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

func TestNewCertConfigPublicKey(t *testing.T) {
	ca, err := securelink.NewCA(nil, "ca")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name   string
		Exist  bool
		Type   securelink.KeyType
		Length securelink.KeyLength
	}{
		{"Present but no values", true, "", ""},
		{"RSA no length", true, securelink.KeyTypeRSA, ""},
		{"Ec no length", true, securelink.KeyTypeEc, ""},
		{"No Type 256", true, "", securelink.KeyLengthEc256},
		{"No Type 2048", true, "", securelink.KeyLengthRsa2048},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			certConfig := securelink.NewDefaultCertificationConfigWithDefaultTemplate(ca, "testing")
			if test.Exist {
				certConfig.PublicKey = &securelink.KeyPair{
					Type:   test.Type,
					Length: test.Length,
				}
			}

			_, err = ca.NewCert(certConfig)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetSignatureAlgorithm(t *testing.T) {
	tests := []struct {
		Name     string
		Type     securelink.KeyType
		Length   securelink.KeyLength
		Expected x509.SignatureAlgorithm
	}{
		{"SHA256WithRSAPSS", securelink.KeyTypeRSA, securelink.KeyLengthRsa2048, x509.SHA256WithRSAPSS},
		{"SHA384WithRSAPSS", securelink.KeyTypeRSA, securelink.KeyLengthRsa3072, x509.SHA384WithRSAPSS},
		{"SHA512WithRSAPSS 4096", securelink.KeyTypeRSA, securelink.KeyLengthRsa4096, x509.SHA512WithRSAPSS},
		{"SHA512WithRSAPSS 8192", securelink.KeyTypeRSA, securelink.KeyLengthRsa8192, x509.SHA512WithRSAPSS},
		{"ECDSAWithSHA256", securelink.KeyTypeEc, securelink.KeyLengthEc256, x509.ECDSAWithSHA256},
		{"ECDSAWithSHA384", securelink.KeyTypeEc, securelink.KeyLengthEc384, x509.ECDSAWithSHA384},
		{"ECDSAWithSHA512", securelink.KeyTypeEc, securelink.KeyLengthEc521, x509.ECDSAWithSHA512},
		{"UnknownSignatureAlgorithm", "", "", x509.UnknownSignatureAlgorithm},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ret := securelink.GetSignatureAlgorithm(test.Type, test.Length)

			if !reflect.DeepEqual(ret, test.Expected) {
				t.Fatalf("the expected signature is %v but had %v", test.Expected, ret)
			}
		})
	}
}
