package echohandler_test

import (
	"crypto/tls"
	"io/ioutil"
	"testing"

	"github.com/labstack/echo"

	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/handlers/echohandler"
)

func TestMain(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ := securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert, _ := ca.NewCert(conf, "1")

	addr := new(addr)
	echoService, err := echohandler.New(addr, "echo", securelink.GetBaseTLSConfig("1", cert))
	if err != nil {
		t.Fatal(err)
	}

	echoService.Echo.GET("/", func(c echo.Context) error {
		c.String(200, "OK")
		return nil
	})

	go echoService.Start()

	getNameFn := func(s string) string {
		return securelink.GetID(s, cert)
	}

	tlsConfig := securelink.GetBaseTLSConfig("1", cert)
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	s, err := securelink.NewServer(1364, tlsConfig, cert, getNameFn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.RegisterService(echoService)

	cli := securelink.NewHTTPSConnector("echo.1", cert)
	resp, err := cli.Get("https://127.0.0.1:1364/")
	if err != nil {
		t.Fatal(err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(buf) != "OK" {
		t.Fatalf("the returned body is %q but not %q", string(buf), "OK")
	}

	if testing.Verbose() {
		t.Logf("the server respond %q", string(buf))
	}
}

type (
	addr struct{}
)

func (a *addr) Network() string {
	return ""
}

func (a *addr) String() string {
	return ":1364"
}
