package securelink_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/alexandre/securelink"
	"github.com/labstack/echo"
)

func Example_serverService() {
	// Build the CA
	conf := securelink.NewDefaultCertificationConfigWithDefaultTemplate("ca")
	ca, _ := securelink.NewCA(conf, "ca")

	// Build the server and the client certificates
	serverCert, _ := ca.NewCert(nil, "server", "localhost")
	clientCert, _ := ca.NewCert(nil, "client")

	// Build a TLS configuration based with the server certificate elements
	tlsConf := securelink.GetBaseTLSConfig("localhost", serverCert)
	server, _ := securelink.NewServer(3815, tlsConf, serverCert, nil)
	defer server.Close()

	// Defines the service matcher function
	testPrefixFn := func(s string) bool {
		if s[:8] == "connTest" {
			return true
		}
		return false
	}
	// Set the handler.
	// In this case it is an echo service but it's here where to do the work for the given service
	handler := func(conn net.Conn) error {
		buff := make([]byte, 24)
		n, _ := conn.Read(buff)
		buff = buff[:n]

		conn.Write(buff)
		return conn.Close()
	}

	// Register the service with it's matcher function and the handler
	server.RegisterService(securelink.NewHandler("connTest", testPrefixFn, handler))

	// Start an other server which in this case will be used only as client for the example
	clientServer, _ := securelink.NewServer(3816, tlsConf, clientCert, nil)
	defer clientServer.Close()

	// Dial to the server and connect to the service "connTest" which is a simple echo service
	secureConn, _ := clientServer.Dial(server.Addr().String(), "connTest", time.Second*1)

	// Client send the content to the given service on the given server
	secureConn.Write([]byte("Hello service"))
	buff := make([]byte, 24)
	// Than read the response
	n, _ := secureConn.Read(buff)
	buff = buff[:n]

	fmt.Println(string(buff))

	// Start a new service connector but not from an other server
	secureConn, _ = securelink.NewServiceConnector(server.Addr().String(), "connTest.localhost", clientCert, time.Second)

	// Client send the content to the given service on the given server
	secureConn.Write([]byte("Hello service BIS"))
	buff = make([]byte, 24)
	// Than read the response
	n, _ = secureConn.Read(buff)
	buff = buff[:n]

	fmt.Println(string(buff))

	// Output:
	// Hello service
	// Hello service BIS
}

func Example_serverHTTP() {
	// Build the CA
	conf := securelink.NewDefaultCertificationConfigWithDefaultTemplate("ca")
	ca, _ := securelink.NewCA(conf, "ca")

	// Build the server and the client certificates
	serverCert, _ := ca.NewCert(nil, "server")
	clientCert, _ := ca.NewCert(nil, "client")

	// Build a TLS configuration based with the server certificate elements
	tlsConf := securelink.GetBaseTLSConfig("localhost", serverCert)
	server, _ := securelink.NewServer(3815, tlsConf, serverCert, nil)
	defer server.Close()

	// Set some values on the HTTP (fallover) service
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

func Example_buildPKI() {
	// Build the CA
	conf := securelink.NewDefaultCertificationConfigWithDefaultTemplate("ca")
	ca, _ := securelink.NewCA(conf, "ca")

	// Build the server certificate
	serverCert, _ := ca.NewCert(nil, "localhost")

	// Extracting certificate and key
	ioutil.WriteFile("cert.pem", serverCert.GetCertPEM(), 0600)
	defer os.RemoveAll("cert.pem")
	ioutil.WriteFile("key.pem", serverCert.GetKeyPEM(), 0600)
	defer os.RemoveAll("key.pem")

	// Add an handler just to do something
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "Hello, TLS!\n")
	})

	// Start the server with the saved keys
	go http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", nil)

	// Start a client using the standard http.Client
	cli := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,

			TLSClientConfig: securelink.GetBaseTLSConfig("localhost", serverCert),
		},
	}

	resp, _ := cli.Get("https://localhost:8443")

	buff, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(buff))

	// Output:
	// Hello, TLS!
}
