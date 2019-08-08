package securelink

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/labstack/echo"
	"github.com/lucas-clemente/quic-go"
	"golang.org/x/crypto/blake2b"

	"github.com/sirupsen/logrus"

	"github.com/alexandrestein/securelink/common"
)

var (
	serviceNameSize = 8
)

type (
	// Server provides a good way to have many services on a single open port.
	// Regester services which are specified with a tls host name prefix.
	Server struct {
		AddrStruct       *common.Addr
		Listeners        []quic.Listener
		packetConns      []*net.UDPConn
		Certificate      *Certificate
		TLSConfig        *tls.Config
		ServiceListeners map[string]*localListener

		Logger *logrus.Logger

		ctx context.Context

		Sessions map[string]quic.Session

		Memberlist *memberlist.Memberlist

		lock *sync.RWMutex
	}
)

// NewServer builds a new server. Provide the port you want the server to listen on.
// The TLS configuration you want to use with a certificate pointer.
// getHostNameFromAddr is a function which gets the remote server hostname.
// This will be used to check the certificate name the server is giving.
func NewServer(ctx context.Context, port uint16, tlsConfig *tls.Config, cert *Certificate) (*Server, error) {
	addr, err := common.NewAddr(port)
	if err != nil {
		return nil, err
	}

	logger := &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Level: logrus.InfoLevel,
	}

	quicConfig := &quic.Config{
		KeepAlive: true,
	}

	s := &Server{
		AddrStruct:       addr,
		Listeners:        make([]quic.Listener, 0),
		packetConns:      make([]*net.UDPConn, 0),
		Certificate:      cert,
		TLSConfig:        tlsConfig,
		ServiceListeners: make(map[string]*localListener),

		Logger: logger,

		ctx: ctx,

		Sessions: make(map[string]quic.Session),

		lock: &sync.RWMutex{},
	}

	// var packetConn net.PacketConn
	// packetConn, err = net.ListenPacket("udp", addr.ForListenerBroadcast())
	// if err != nil {
	// 	return nil, err
	// }

	// var quicListener quic.Listener
	// quicListener, err = quic.Listen(packetConn, tlsConfig, quicConfig)
	// // quicListener, err = quic.Listen(addr.ForListenerBroadcast(), tlsConfig, quicConfig)
	// if err != nil {
	// 	return nil, err
	// }

	var ok bool

	// Clean up listeners if there's an error.
	defer func() {
		if !ok {
			for i := range s.Listeners {
				go s.Listeners[i].Close()
				go s.packetConns[i].Close()
			}
		}
	}()

	// Build all the QUIC listeners.
	for _, addr := range addr.Addrs {
		// var packetConn net.PacketConn
		// packetConn, err = net.ListenPacket("udp", fmt.Sprintf("%s:%d", addr, port))
		ip := net.ParseIP(addr)
		udpAddr := &net.UDPAddr{IP: ip, Port: int(port)}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			return nil, err
		}

		var quicListener quic.Listener
		quicListener, err = quic.Listen(udpConn, tlsConfig, quicConfig)
		if err != nil {
			return nil, err
		}

		s.Listeners = append(s.Listeners, quicListener)
		s.packetConns = append(s.packetConns, udpConn)

		// Fire them up now that we've been able to create them all.
		go func() {
			for {
				sess, err := quicListener.Accept(ctx)
				if err != nil {
					if err.Error() == "server closed" {
						s.Logger.Infoln("quic listener is closed")
						return
					}
					continue
				}
				s.lock.Lock()
				s.Sessions[sess.RemoteAddr().String()] = sess
				s.lock.Unlock()
				go s.handleConn(sess)
			}
		}()
	}

	ok = true

	s.Logger.Infof("server started on port %d", addr.Port)

	return s, nil
}

func (s *Server) handleConn(sess quic.Session) {
	// Remove the session from the active session map.
	rmSessionFn := func() {
		s.lock.Lock()
		delete(s.Sessions, sess.RemoteAddr().String())
		s.lock.Unlock()
	}
	defer rmSessionFn()

	for {
		str, err := sess.AcceptStream(s.ctx)
		if err != nil {
			if err.Error() != "NO_ERROR" {
				s.Logger.Debugln("Accepting stream failed:", err)
			}
			sess.Close()
			return
		}

		var target string
		target, err = s.getTarget(str)
		if err != nil {
			s.Logger.Debugln("Accepting getting target:", err)
			sess.Close()
			return
		}

		s.lock.RLock()
		listener := s.ServiceListeners[target]
		s.lock.RUnlock()
		if listener == nil {
			s.Logger.Debugf("target listener %s is nil", target)
			sess.Close()
			return
		}

		go func() {
			conn := &localConn{
				Stream:  str,
				session: sess,
			}
			select {
			case listener.connChan <- conn:
			case <-s.ctx.Done():
				return
			}
		}()
	}
}

// reads the first serviceNameSize bytes length to get the needes info
// and returns the service name which is targedted
func (s *Server) getTarget(str quic.Stream) (string, error) {
	checkBuff := make([]byte, serviceNameSize)
	_, err := str.Read(checkBuff)
	if err != nil {
		return "", err
	}

	ret := hex.EncodeToString(checkBuff)

	return ret, nil
}

// Close implements the net.Listener interface
func (s *Server) Close() {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for _, listener := range s.ServiceListeners {
		go listener.Close()
	}
	for _, session := range s.Sessions {
		go session.Close()
	}
	for _, listener := range s.Listeners {
		go listener.Close()
	}
}

// Dial is used to connect to on other server and set a prefix to access specific registered service
func (s *Server) Dial(addr net.Addr, serviceName string, timeout time.Duration) (net.Conn, error) {
	session, err := s.dial(addr, timeout)
	if err != nil {
		return nil, err
	}

	stream, err := session.OpenStream()
	if err != nil {
		return nil, err
	}

	stream.SetDeadline(
		time.Now().Add(timeout),
	)

	hash, err := blake2b.New(serviceNameSize, []byte{})
	if err != nil {
		return nil, err
	}
	hash.Write([]byte(serviceName))
	key := hash.Sum(nil)

	_, err = stream.Write(key)
	if err != nil {
		return nil, err
	}

	conn := &localConn{
		Stream:  stream,
		session: session,
	}

	return conn, nil
}

func (s *Server) dial(addr net.Addr, timeout time.Duration) (quic.Session, error) {
	s.lock.RLock()
	session := s.Sessions[addr.String()]
	s.lock.RUnlock()
	if session != nil {
		return session, nil
	}

newConn:

	session, err := s.dialQuic(s.ctx, s.TLSConfig, addr, "", timeout)
	if err != nil {
		return nil, err
	}

	s.lock.Lock()
	s.Sessions[addr.String()] = session
	s.lock.Unlock()

	go func() {
		<-session.Context().Done()
		s.lock.Lock()
		delete(s.Sessions, addr.String())
		s.lock.Unlock()
	}()

	err = session.Context().Err()
	if err != nil {
		session = nil
		goto newConn
	}

	return session, nil
}

func (s *Server) NewListener(name string) (net.Listener, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	hash, err := blake2b.New(serviceNameSize, []byte{})
	if err != nil {
		return nil, err
	}
	hash.Write([]byte(name))
	key := hash.Sum(nil)
	keyAsString := hex.EncodeToString(key)

	existingListener := s.ServiceListeners[keyAsString]
	if existingListener != nil {
		return nil, fmt.Errorf("a listener with the same name is already registered")
	}

	ll := &localListener{
		name:     name,
		server:   s,
		connChan: make(chan net.Conn, 32),
	}

	s.ServiceListeners[keyAsString] = ll

	return ll, nil
}

func (s *Server) Ctx() context.Context {
	return s.ctx
}

type (
	localConn struct {
		quic.Stream
		session quic.Session
	}

	localListener struct {
		name     string
		server   *Server
		connChan chan net.Conn
	}
)

func (lc *localConn) LocalAddr() net.Addr {
	return lc.session.LocalAddr()
}
func (lc *localConn) RemoteAddr() net.Addr {
	return lc.session.RemoteAddr()
}

func (ll *localListener) Accept() (net.Conn, error) {
	conn, ok := <-ll.connChan
	if !ok {
		return nil, fmt.Errorf("the listener looks close")
	}
	return conn, nil
}

func (ll *localListener) Close() error {
	ll.server.lock.Lock()
	defer ll.server.lock.Unlock()

	hash, _ := blake2b.New(serviceNameSize, nil)
	hash.Write([]byte(ll.name))
	listenerKey := hash.Sum(nil)

	listenerKeyAsHex := hex.EncodeToString(listenerKey)

	delete(ll.server.ServiceListeners, listenerKeyAsHex)
	return nil
}

func (ll *localListener) Addr() net.Addr {
	return ll.server.Listeners[0].Addr()
}

// GetBaseTLSConfig returns a TLS configuration with the given certificate as
// "Certificate" and setup the "RootCAs" with the given certificate CertPool
func GetBaseTLSConfig(host string, cert *Certificate) *tls.Config {
	return &tls.Config{
		ServerName:   host,
		Certificates: []tls.Certificate{cert.GetTLSCertificate()},
		RootCAs:      cert.GetCertPool(),
		ClientCAs:    cert.GetCertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
		NextProtos:   []string{"securelink"},
	}
}

// NewHTTPEchoServer prepar a Echo server. To start it you need to run:
// e.StartServer(&http.Server{}) after you have set the routes.
func NewHTTPEchoServer(ln net.Listener) *echo.Echo {
	e := echo.New()

	e.Listener = ln
	e.HideBanner = true
	e.HidePort = true

	return e
}

func (s *Server) dialQuic(ctx context.Context, tlsConfig *tls.Config, addr net.Addr, hostname string, timeout time.Duration) (quic.Session, error) {
	tlsConfigClone := tlsConfig.Clone()

	quicConfig := new(quic.Config)
	if timeout != 0 {
		quicConfig = &quic.Config{
			HandshakeTimeout: timeout,
		}
	}

	sess, err := quic.DialContext(ctx, s.packetConns[0], addr, hostname, tlsConfigClone, quicConfig)
	if err != nil {
		// fmt.Println("1", s.packetConns[0].LocalAddr(), err)
		return nil, err
	}
	// fmt.Println("local", s.packetConn.LocalAddr())
	return sess, nil
	// return quic.DialAddrContext(ctx, addr, tlsConfigClone, quicConfig)
}

// func (pc *mockPacketConn) LocalAddr() net.Addr {
// 	fmt.Println("get local", pc.addr.String(), pc.PacketConn.LocalAddr())
// 	return pc.addr
// }

// type (
// 	mockPacketConn struct {
// 		net.PacketConn
// 		addr *common.Addr
// 	}
// )
