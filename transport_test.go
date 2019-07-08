package securelink_test

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo"

	"github.com/alexandrestein/gotinydb/replication/securelink"
)

const (
	secret1 = "secret1"
	secret2 = "secret2"
)

var (
	s1, s2 *securelink.Server
	tt     *testing.T

	ca *securelink.Certificate
)

func TestTransportAndServer(t *testing.T) {
	tt = t

	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	ca, _ = securelink.NewCA(conf, "ca")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert1, _ := ca.NewCert(conf, "1")

	conf = securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)
	cert2, _ := ca.NewCert(conf, "2")

	getHostNameFunc := func(addr string) (serverID string) {
		return securelink.GetID(addr, ca)
	}

	var err error
	s1, err = securelink.NewServer(3461, securelink.GetBaseTLSConfig("1", cert1), cert1, getHostNameFunc)
	if err != nil {
		t.Fatal(err)
	}
	s2, err = securelink.NewServer(3462, securelink.GetBaseTLSConfig("2", cert2), cert2, getHostNameFunc)
	if err != nil {
		t.Fatal(err)
	}

	testPrefixFn := func(s string) bool {
		if len(s) < 4 {
			return false
		}
		if s[:4] == "test" {
			return true
		}
		return false
	}
	s1.RegisterService(securelink.NewHandler("testGroup", testPrefixFn, handle1))
	s2.RegisterService(securelink.NewHandler("testGroup", testPrefixFn, handle2))

	var conn net.Conn
	conn, err = s2.Dial(":3461", "test", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Connect, send a small message and read the response
	conn, err = s2.Dial(":3461", "test", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	var n int
	n, err = conn.Write([]byte(secret1))
	if err != nil {
		t.Fatal(err)
	}
	if testing.Verbose() {
		t.Logf("the client has write %d bytes to server: %s", n, secret1)
	}

	buff := make([]byte, 150)
	n, err = conn.Read(buff)
	if err != nil {
		t.Fatal(err)
	}
	buff = buff[:n]

	if string(buff) != secret2 {
		t.Fatalf("the returned secret is not good")
	}

	if testing.Verbose() {
		t.Logf("the client has read %d bytes from server: %s", n, string(buff))
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	// t.Run("net.Listener interface", testNetListenerInterface)
	t.Run("deregister", testDeregister)
	t.Run("http fallback", httpFallback)

	err = s1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = s2.Close()
	if err != nil {
		t.Fatal(err)
	}
}

// Accept a connection and contact the other server to get the second secret and return the second secret
// to the first one.
func handle1(connAsServer net.Conn) error {
	buf := make([]byte, 100)
	n, err := connAsServer.Read(buf)
	if err != nil {
		if err == io.EOF {
			return err
		}
		tt.Fatal(err)
	}

	cs := connAsServer.(*securelink.TransportConn).ConnectionState()

	remoteClientServerName := cs.ServerName

	var connAsClient net.Conn
	connAsClient, err = s1.Dial(":3462", "test", time.Millisecond*500)
	if err != nil {
		tt.Fatal(err)
	}
	defer connAsClient.Close()

	remoteServerServerName := cs.ServerName

	if remoteClientServerName != remoteServerServerName {
		tt.Fatalf("the connected client and the corresponding server are not corresponding %s != %s", remoteClientServerName, remoteServerServerName)
	}

	_, err = connAsClient.Write(buf[:n])
	if err != nil {
		tt.Fatal(err)
	}

	buf2 := make([]byte, 100)
	n, err = connAsClient.Read(buf2)
	if err != nil {
		tt.Fatal(err)
	}

	_, err = connAsServer.Write(buf2[:n])
	if err != nil {
		tt.Fatal(err)
	}

	return nil
}

// Check that the client sent secret one and returns secret 2
func handle2(connAsServer net.Conn) error {
	buf := make([]byte, 100)
	n, err := connAsServer.Read(buf)
	if err != nil {
		tt.Fatal(err)
	}

	if string(buf[:n]) != secret1 {
		tt.Fatalf("bad secret %s, %d", buf[:n], n)
	}

	_, err = connAsServer.Write([]byte(secret2))
	if err != nil {
		tt.Fatal(err)
	}

	return nil
}

func httpFallback(t *testing.T) {
	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

	srvCert, _ := ca.NewCert(conf, "srv")
	cliCert, _ := ca.NewCert(conf, "cli")

	s, err := securelink.NewServer(7777, securelink.GetBaseTLSConfig("", srvCert), srvCert, nil)
	if err != nil {
		t.Fatal(err)
	}

	s.Echo.GET("/", func(c echo.Context) error {
		return c.String(200, "OK")
	})

	// time.Sleep(time.Minute)

	cli := securelink.NewHTTPSConnector("srv", cliCert)
	var resp *http.Response
	resp, err = cli.Get("https://127.0.0.1:7777/")
	if err != nil {
		t.Fatal(err)
	}

	var buf []byte
	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(buf) != "OK" {
		t.Fatalf("the replied value is not what we expect: %q instead of %q", string(buf), "OK")
	}
}

func testDeregister(t *testing.T) {
	s1.DeregisterService("testGroup")

	conf := securelink.NewDefaultCertificationConfig()
	conf.CertTemplate = securelink.GetCertTemplate(nil, nil)

	cert, _ := ca.NewCert(conf, "cli")
	conn, err := securelink.NewServiceConnector(":3461", "test.1", cert, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 100)
	var n int
	n, _ = conn.Read(buf)
	if n == 7 {
		t.Fatalf("the service must be deregister and connection should retrun an http respond as fallback but read: %q", string(buf[:n]))
	}
}

func TestBaseListener(t *testing.T) {
	bl := securelink.NewBaseListener(nil)

	go func() {
		bl.AcceptChan <- nil
	}()

	_, err := bl.Accept()
	if err != nil {
		t.Fatal(err)
	}

	bl.Addr()
	bl.Close()
}
