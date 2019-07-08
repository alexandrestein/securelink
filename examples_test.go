package securelink_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"gitea.interlab-net.com/alexandre/securelink"
	"github.com/labstack/echo"
)

func ExampleServerService() {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ = securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	serverCert, _ := ca.NewCert(conf, "server")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	clientCert, _ := ca.NewCert(conf, "client")

	tlsConf := securelink.GetBaseTLSConfig("localhost", serverCert)
	server, _ := securelink.NewServer(3815, tlsConf, serverCert, nil)
	defer server.Close()

	testPrefixFn := func(s string) bool {
		if s[:8] == "connTest" {
			return true
		}
		return false
	}
	handler := func(conn net.Conn) error {
		buff := make([]byte, 24)
		n, _ := conn.Read(buff)
		buff = buff[:n]

		conn.Write(buff)
		return conn.Close()
	}

	server.RegisterService(securelink.NewHandler("connTest", testPrefixFn, handler))

	clientServer, _ := securelink.NewServer(3816, tlsConf, clientCert, nil)
	defer clientServer.Close()

	secureConn, _ := clientServer.Dial(server.Addr().String(), "connTest", time.Second*1)

	secureConn.Write([]byte("Hello ECHO"))
	buff := make([]byte, 24)
	n, _ := secureConn.Read(buff)
	buff = buff[:n]

	fmt.Println(string(buff))

	securelink.NewHTTPSConnector("localhost:3815", clientCert)

	// Output:
	// Hello ECHO
}

func ExampleServerHTTP() {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ = securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	serverCert, _ := ca.NewCert(conf, "server")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	clientCert, _ := ca.NewCert(conf, "client")

	tlsConf := securelink.GetBaseTLSConfig("localhost", serverCert)
	server, _ := securelink.NewServer(3815, tlsConf, serverCert, nil)
	defer server.Close()

	server.Echo.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	cli := securelink.NewHTTPSConnector("server", clientCert)
	resp, _ := cli.Get("https://localhost:3815/")

	buff, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(buff))
	// Output:
	// Hello, World!
}
