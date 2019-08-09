package securelink_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/alexandrestein/securelink"
)

func initServers(ctx context.Context, n, portOffset int) (ca *securelink.Certificate, servers []*securelink.Server) {
	conf := securelink.NewDefaultCertificationConfig()
	ca, _ = securelink.NewCA(conf, "ca")

	servers = make([]*securelink.Server, n)
	for i := range servers {
		conf = securelink.NewDefaultCertificationConfig()
		conf.IsCA = true
		cert, _ := ca.NewCert(conf)
		server, _ := securelink.NewServer(ctx, 3160+uint16(i)+uint16(portOffset), securelink.GetBaseTLSConfig(fmt.Sprint(i), cert), cert)

		if testing.Verbose() {
			server.Logger.SetLevel(logrus.TraceLevel)
		}

		servers[i] = server
	}

	return
}

func echoFn(ctx context.Context, t *testing.T, listener net.Listener) {
	echoChan := make(chan net.Conn)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				t.Error(err)
				return
			}
			echoChan <- conn

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	for {
		var conn net.Conn
		select {
		case <-ctx.Done():
			return
		case conn = <-echoChan:
		}

		defer conn.Close()

		buff := make([]byte, 1024)

		var n int
		n, err := conn.Read(buff)
		if err != nil {
			t.Error(err)
			return
		}

		buff = buff[:n]
		conn.Write(buff)
	}
}

func TestServerAndClosedListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, servers := initServers(ctx, 2, 0)

	s0 := servers[0]
	defer s0.Close()

	s1 := servers[1]
	defer s1.Close()

	serviceName := "echo service"
	listener, err := s0.NewListener(serviceName)
	defer listener.Close()

	go echoFn(ctx, t, listener)

	time.Sleep(time.Millisecond * 100)

	// Try on open listener
	conn, err := s1.Dial(s0.AddrStruct.MustUDPAddr(), serviceName, time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	buff := []byte("HELLO")
	n, err := conn.Write(buff)
	if err != nil {
		t.Error(err)
		return
	}
	if n != len(buff) {
		t.Errorf("written expected %d but had %d", len(buff), n)
		return
	}

	time.Sleep(time.Millisecond * 500)

	buff2 := make([]byte, 1024)
	n, err = conn.Read(buff2)
	if err != nil {
		t.Error(err)
		return
	}

	buff2 = buff2[:n]
	if testing.Verbose() {
		t.Logf("the returned message is: %q", string(buff2))
	}

	if n != len(buff) {
		t.Errorf("written expected %d but had %d", len(buff), n)
		return
	}

	if !bytes.Equal(buff, buff2) {
		t.Errorf("Response is not equal. Expected %q but had %q", string(buff), string(buff2))
		return
	}

	err = conn.Close()
	if err != nil {
		t.Error(err)
		return
	}

	err = listener.Close()
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Millisecond * 100)

	// Try on a closed listener
	conn, err = s1.Dial(s0.AddrStruct.MustUDPAddr(), serviceName, time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	n, err = conn.Write([]byte("test transmission"))
	if err != nil {
		t.Error(err)
		return
	}

	n, err = conn.Read(make([]byte, 1024))
	if err == nil {
		t.Errorf("No listener so the read should get an error")
		return
	}
	if n != 0 {
		t.Errorf("read should be zero but had: %d", n)
		return
	}

	s0.Close()
	s1.Close()
}

func TestServerBadCertificates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, servers := initServers(ctx, 1, 0)

	s0 := servers[0]
	defer s0.Close()

	_, servers = initServers(ctx, 1, 1)
	s1 := servers[0]
	defer s1.Close()

	serviceName := "echo service"
	listener, _ := s0.NewListener(serviceName)
	defer listener.Close()

	go echoFn(ctx, t, listener)

	time.Sleep(time.Millisecond * 100)

	addr, _ := net.ResolveUDPAddr("udp", "localhost:3160")
	_, err := s1.Dial(addr, serviceName, time.Second)
	if err == nil {
		t.Errorf("dial must return an error because the certificate authority in not the same")
		return
	}
}
