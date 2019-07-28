package securelink

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

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

		Sessions map[string]quic.Session

		getHostNameFromAddr FuncGetHostNameFromAddr

		lock *sync.Mutex
	}

	closedConn struct{}
)

// NewServer builds a new server. Provide the port you want the server to listen on.
// The TLS configuration you want to use with a certificate pointer.
// getHostNameFromAddr is a function which gets the remote server hostname.
// This will be used to check the certificate name the server is giving.
func NewServer(port uint16, tlsConfig *tls.Config, cert *Certificate, getHostNameFromAddr FuncGetHostNameFromAddr) (*Server, error) {
	addr, err := common.NewAddr(port)
	if err != nil {
		return nil, err
	}

	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2", "http/1.1")

	// var tlsListener net.Listener
	// tlsListener, err = tls.Listen("tcp", addr.ForListenerBroadcast(), tlsConfig)

	var quicListener quic.Listener
	quicListener, err = quic.ListenAddr(addr.ForListenerBroadcast(), tlsConfig, nil)

	if err != nil {
		return nil, err
	}

	if getHostNameFromAddr == nil {
		getHostNameFromAddr = func(s string) string {
			return GetID(s, cert)
		}
	}

	s := &Server{
		AddrStruct:  addr,
		Listener:    quicListener,
		Certificate: cert,
		TLSConfig:   tlsConfig,
		Listeners:   make(map[string]*localListener),

		Sessions: make(map[string]quic.Session),

		getHostNameFromAddr: getHostNameFromAddr,

		lock: &sync.Mutex{},
	}

	// quic.ListenAndServeTLSConfig(addr.ForListenerBroadcast(), tlsConfig, s.Echo.h)

	go func() {
		for {
			sess, err := s.Listener.Accept()
			if err != nil {
				fmt.Println("error in accept", err)
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
			fmt.Println("Accepting stream failed:", err)
			sess.Close()
			return
		}

		var target string
		target, err = s.getTarget(str)
		if err != nil {
			fmt.Println("Accepting getting target:", err)
			sess.Close()
			return
		}

		listener := s.Listeners[target]
		if listener == nil {
			fmt.Println("not target found", s.Listeners, target, len(target))
			sess.Close()
			return

		}

		go func() {
			// ctx, cancel := context.WithCancel(sess.Context())
			// defer cancel()

			conn := &localConn{
				// ctx:     ctx,
				Stream:  str,
				session: sess,
			}
			listener.connChan <- conn

			// buff := make([]byte, 1024)
			// str.Read(buff)
			// fmt.Println("handler needs to be made", string(buff))
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
	return s.Listener.Close()
}

// Dial is used to connect to on other server and set a prefix to access specific registered service
func (s *Server) Dial(addr, serviceName string, timeout time.Duration) (net.Conn, error) {
	session := s.Sessions[addr]
newConn:
	if session == nil {
		hostName := s.getHostNameFromAddr(addr)

		// if serviceName != "" {
		// 	hostName = fmt.Sprintf("%s.%s", serviceName, hostName)
		// }

		tlsConfig := &tls.Config{}
		*tlsConfig = *s.TLSConfig
		tlsConfig.ServerName = hostName

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		var err error
		session, err = quic.DialAddrContext(ctx, addr, tlsConfig, nil)
		if err != nil {
			return nil, err
		}

		s.Sessions[addr] = session
	} else {
		err := session.Context().Err()
		if err != nil {
			session = nil
			goto newConn
		}
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

	// buff := []byte(name)
	// l := len(buff)
	// if l > 32 {
	// 	return nil, fmt.Errorf("the name is too long")
	// }

	// zeroBuff := make([]byte, serviceNameSize-l)
	// buff = append(buff, zeroBuff...)
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

type (
	localConn struct {
		// ctx context.Context
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
	fmt.Println("asked for close")
	ll.server.lock.Lock()
	defer ll.server.lock.Unlock()

	hash, _ := blake2b.New(serviceNameSize, nil)
	hash.Write([]byte(ll.name))
	listenerKey := hash.Sum(nil)

	listenerKeyAsHex := hex.EncodeToString(listenerKey)

	delete(ll.server.Listeners, listenerKeyAsHex)
	fmt.Println("closed")
	return nil
}

func (ll *localListener) Addr() net.Addr {
	return ll.server.Listener.Addr()
}
