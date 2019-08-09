package securelink

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

const (
	tcpServiceName    = "TCP memberlist"
	udpServiceName    = "UDP memberlist"
	quicPacketBufSize = 2 * 1024 * 1024
	quicTimeout       = 1 * time.Second
)

// netTransport is a Transport implementation that uses connectionless UDP for
// packet operations, and ad-hoc TCP connections for stream operations.
type netTransport struct {
	// config   *netTransportConfig
	packetCh    chan *memberlist.Packet
	streamCh    chan net.Conn
	logger      *logrus.Logger
	wg          sync.WaitGroup
	server      *Server
	tcpListener net.Listener
	udpListener net.Listener
	shutdown    int32
}

func (s *Server) StartMemberlist(config *memberlist.Config) error {
	if config == nil {
		return fmt.Errorf("config can't be nil")
	}

	tr, err := newNetTransport(s)
	if err != nil {
		return err
	}
	config.Transport = tr

	config.BindAddr = s.AddrStruct.MainAddr()
	config.BindPort = int(s.AddrStruct.Port())
	config.AdvertiseAddr = s.AddrStruct.MainAddr()
	config.AdvertisePort = int(s.AddrStruct.Port())

	config.EnableCompression = true

	mb, err := memberlist.Create(config)
	if err != nil {
		return err
	}

	s.Memberlist = mb
	return nil
}

// NewnetTransport returns a net transport with the given configuration. On
// success all the network listeners will be created and listening.
func newNetTransport(server *Server) (*netTransport, error) {
	tcpLn, err := server.NewListener(tcpServiceName)
	if err != nil {
		return nil, fmt.Errorf("Failed to start TCP listener: %v", err)
	}

	udpLn, err := server.NewListener(udpServiceName)
	if err != nil {
		return nil, fmt.Errorf("Failed to start UDP listener: %v", err)
	}

	// Build out the new transport.
	t := netTransport{
		packetCh:    make(chan *memberlist.Packet),
		streamCh:    make(chan net.Conn),
		logger:      server.Logger,
		tcpListener: tcpLn,
		udpListener: udpLn,
		server:      server,
	}

	// Fire up now that we've been able to create it.
	t.wg.Add(2)
	go t.tcpListen(tcpLn)
	go t.udpListen(udpLn)

	return &t, nil
}

// GetAutoBindPort returns the bind port that was automatically given by the
// kernel, if a bind port of 0 was given.
func (t *netTransport) GetAutoBindPort() int {
	// We made sure there's at least one TCP listener, and that one's
	// port was applied to all the others for the dynamic bind case.
	return int(t.server.AddrStruct.Port())
}

// See Transport.
func (t *netTransport) FinalAdvertiseAddr(ip string, port int) (net.IP, int, error) {
	var advertiseAddr net.IP
	var advertisePort int

	advertiseAddr = net.ParseIP(t.server.AddrStruct.MainAddr())
	if advertiseAddr == nil {
		return nil, 0, fmt.Errorf("Failed to parse advertise address: %q", ip)
	}

	// Use the port we are bound to.
	advertisePort = t.GetAutoBindPort()

	return advertiseAddr, advertisePort, nil
}

// See Transport.
func (t *netTransport) WriteTo(b []byte, addr string) (time.Time, error) {
	// We made sure there's at least one UDP listener, so just use the
	// packet sending interface on the first one. Take the time after the
	// write call comes back, which will underestimate the time a little,
	// but help account for any delays before the write occurs.
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return time.Time{}, err
	}

	var conn net.Conn
	conn, err = t.server.Dial(udpAddr, udpServiceName, quicTimeout)
	if err != nil {
		return time.Time{}, err
	}

	_, err = conn.Write(b)
	if err != nil {
		return time.Time{}, err
	}

	return time.Now(), err
}

// See Transport.
func (t *netTransport) PacketCh() <-chan *memberlist.Packet {
	return t.packetCh
}

// See Transport.
func (t *netTransport) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := t.server.Dial(udpAddr, tcpServiceName, timeout)
	if err != nil {
		fmt.Println("dial err", err)
		return nil, err
	}

	return conn, nil
}

// See Transport.
func (t *netTransport) StreamCh() <-chan net.Conn {
	return t.streamCh
}

// See Transport.
func (t *netTransport) Shutdown() error {
	// This will avoid log spam about errors when we shut down.
	atomic.StoreInt32(&t.shutdown, 1)

	t.tcpListener.Close()
	t.udpListener.Close()

	// Block until all the listener threads have died.
	t.wg.Wait()
	return nil
}

// tcpListen is a long running goroutine that accepts incoming TCP connections
// and hands them off to the stream channel.
func (t *netTransport) tcpListen(ln net.Listener) {
	defer t.wg.Done()

	// baseDelay is the initial delay after an AcceptTCP() error before attempting again
	const baseDelay = 5 * time.Millisecond

	// maxDelay is the maximum delay after an AcceptTCP() error before attempting again.
	// In the case that tcpListen() is error-looping, it will delay the shutdown check.
	// Therefore, changes to maxDelay may have an effect on the latency of shutdown.
	const maxDelay = 1 * time.Second

	var loopDelay time.Duration
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s := atomic.LoadInt32(&t.shutdown); s == 1 {
				break
			}

			if loopDelay == 0 {
				loopDelay = baseDelay
			} else {
				loopDelay *= 2
			}

			if loopDelay > maxDelay {
				loopDelay = maxDelay
			}

			t.logger.Printf("[ERR] memberlist: Error accepting connection: %v", err)
			time.Sleep(loopDelay)
			continue
		}
		// No error, reset loop delay
		loopDelay = 0

		t.streamCh <- conn
	}
}

// udpListen is a long running goroutine that accepts incoming UDP packets and
// hands them off to the packet channel.
func (t *netTransport) udpListen(ln net.Listener) {
	defer t.wg.Done()
	for {
		// Do a blocking read into a fresh buffer. Grab a time stamp as
		// close as possible to the I/O.
		buf := make([]byte, quicPacketBufSize)
		conn, err := ln.Accept()
		if err != nil {
			if s := atomic.LoadInt32(&t.shutdown); s == 1 {
				break
			}

			t.logger.Printf("[ERR] memberlist: Error reading UDP packet: %v", err)
			continue
		}

		addr := conn.RemoteAddr()
		var n int
		n, err = conn.Read(buf)
		ts := time.Now()
		if err != nil {
			if s := atomic.LoadInt32(&t.shutdown); s == 1 {
				break
			}

			t.logger.Printf("[ERR] memberlist: Error reading UDP packet: %v", err)
			continue
		}

		// Check the length - it needs to have at least one byte to be a
		// proper message.
		if n < 1 {
			t.logger.Printf("[ERR] memberlist: UDP packet too short (%d bytes) %s",
				len(buf), memberlist.LogAddress(addr))
			continue
		}

		// Ingest the packet.
		metrics.IncrCounter([]string{"memberlist", "udp", "received"}, float32(n))
		t.packetCh <- &memberlist.Packet{
			Buf:       buf[:n],
			From:      addr,
			Timestamp: ts,
		}
	}
}
