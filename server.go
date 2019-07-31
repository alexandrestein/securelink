package securelink

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/labstack/echo"
	"github.com/lucas-clemente/quic-go"
	"golang.org/x/crypto/blake2b"

	"github.com/alexandrestein/securelink/common"
)

var (
	serviceNameSize = 8
)

type (
	// Server provides a good way to have many services on a single open port.
	// Regester services which are specified with a tls host name prefix.
	Server struct {
		AddrStruct  *common.Addr
		Listener    quic.Listener
		Certificate *Certificate
		TLSConfig   *tls.Config
		Listeners   map[string]*localListener

		Logger *log.Logger

		ctx context.Context

		Sessions map[string]quic.Session

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

	logger := log.New(os.Stdout, fmt.Sprintf("securelink-server :%d", port), log.LstdFlags)

	quicConfig := &quic.Config{
		KeepAlive: true,
	}

	var quicListener quic.Listener
	quicListener, err = quic.ListenAddr(addr.ForListenerBroadcast(), tlsConfig, quicConfig)

	if err != nil {
		return nil, err
	}

	s := &Server{
		AddrStruct:  addr,
		Listener:    quicListener,
		Certificate: cert,
		TLSConfig:   tlsConfig,
		Listeners:   make(map[string]*localListener),

		Logger: logger,

		ctx: ctx,

		Sessions: make(map[string]quic.Session),

		lock: &sync.RWMutex{},
	}

	go func() {
		for {
			sess, err := s.Listener.Accept()
			if err != nil {
				if err.Error() == "server closed" {
					return
				}
				continue
			}
			go s.handleConn(sess)
		}
	}()

	return s, nil
}

func (s *Server) handleConn(sess quic.Session) {
	for {
		str, err := sess.AcceptStream()
		if err != nil {
			// fmt.Println("Accepting stream failed:", err)
			sess.Close()
			return
		}

		var target string
		target, err = s.getTarget(str)
		if err != nil {
			// fmt.Println("Accepting getting target:", err)
			sess.Close()
			return
		}

		s.lock.RLock()
		listener := s.Listeners[target]
		s.lock.RUnlock()
		if listener == nil {
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
func (s *Server) Close() error {
	s.lock.RLock()
	listeners := map[string]*localListener{}
	sessions := map[string]quic.Session{}
	listeners, sessions = s.Listeners, s.Sessions
	s.lock.RUnlock()
	for _, listener := range listeners {
		listener.Close()
	}
	for _, session := range sessions {
		session.Close()
	}
	return s.Listener.Close()
}

// Dial is used to connect to on other server and set a prefix to access specific registered service
func (s *Server) Dial(addr, serviceName string, timeout time.Duration) (net.Conn, error) {
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

func (s *Server) dial(addr string, timeout time.Duration) (quic.Session, error) {
	s.lock.RLock()
	session := s.Sessions[addr]
	s.lock.RUnlock()
	if session != nil {
		return session, nil
	}

newConn:
	tlsConfig := s.TLSConfig.Clone()
	// tlsConfig.InsecureSkipVerify = true

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var quicConfig *quic.Config
	if timeout != 0 {
		quicConfig = &quic.Config{
			HandshakeTimeout: timeout,
		}
	}

	session, err := quic.DialAddrContext(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	// if !s.verifyRemoteCert(session.ConnectionState()) {
	// 	session.Close()
	// 	return nil, fmt.Errorf("bad certificate")
	// }

	s.lock.Lock()
	s.Sessions[addr] = session
	s.lock.Unlock()

	go func() {
		<-session.Context().Done()
		s.lock.Lock()
		delete(s.Sessions, addr)
		s.lock.Unlock()
	}()

	err = session.Context().Err()
	if err != nil {
		session = nil
		goto newConn
	}

	return session, nil
}

// func (s *Server) verifyRemoteCert(connState tls.ConnectionState) bool {
// 	verifyOptions := x509.VerifyOptions{
// 		Roots: s.Certificate.GetCertPool(),
// 	}
// 	for _, cert := range connState.PeerCertificates {
// 		_, err := cert.Verify(verifyOptions)
// 		if err != nil {
// 			continue
// 		}

// 		return true
// 	}

// 	return false
// }

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

	existingListener := s.Listeners[keyAsString]
	if existingListener != nil {
		return nil, fmt.Errorf("a listener with the same name is already registered")
	}

	ll := &localListener{
		name:     name,
		server:   s,
		connChan: make(chan net.Conn, 32),
	}

	s.Listeners[keyAsString] = ll

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

	delete(ll.server.Listeners, listenerKeyAsHex)
	return nil
}

func (ll *localListener) Addr() net.Addr {
	return ll.server.Listener.Addr()
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
