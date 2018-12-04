// Package securelink enable caller to stream multiple connection inside a sign
// TLS link. The package provide CA and certificate generation to have easy management.
package securelink

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/alexandrestein/common"
)

type (
	// Server provides a good way to have many services on one sign open port.
	// Regester services which are selected with a tls host name prefix.
	Server struct {
		Echo        *echo.Echo
		AddrStruct  *common.Addr
		TLSListener net.Listener
		Certificate *Certificate
		TLSConfig   *tls.Config
		Handlers    []Handler

		getHostNameFromAddr FuncGetHostNameFromAddr
		errChan             chan error
	}
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

	var tlsListener net.Listener
	tlsListener, err = tls.Listen("tcp", addr.ForListenerBroadcast(), tlsConfig)
	if err != nil {
		return nil, err
	}

	if getHostNameFromAddr == nil {
		getHostNameFromAddr = func(s string) string {
			return GetID(s, cert)
		}
	}

	s := &Server{
		Echo:        echo.New(),
		AddrStruct:  addr,
		TLSListener: tlsListener,
		Certificate: cert,
		TLSConfig:   tlsConfig,
		Handlers:    []Handler{},

		getHostNameFromAddr: getHostNameFromAddr,
		errChan:             make(chan error),
	}

	s.Echo.TLSListener = s
	s.Echo.HideBanner = true
	s.Echo.HidePort = true

	httpServer := new(http.Server)
	httpServer.TLSConfig = tlsConfig
	httpServer.Addr = addr.String()

	go func(e *Server, httpServer *http.Server) {
		s.errChan <- s.Echo.StartServer(httpServer)
	}(s, httpServer)

	return s, nil
}

// Accept implements the net.Listener interface
func (s *Server) Accept() (net.Conn, error) {
waitForNewConn:
	conn, err := s.TLSListener.Accept()
	if err != nil {
		return nil, err
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		goto waitForNewConn
	}

	err = tlsConn.Handshake()
	if err != nil {
		goto waitForNewConn
	}

	tc, _ := newTransportConn(conn, true)

	if tlsConn.ConnectionState().ServerName != "" {
		for _, service := range s.Handlers {
			if service.Match(tc.ConnectionState().ServerName) {
				// Gives the hand to the regestred service
				go service.Handle(tc)
				// And go wait for the next connection
				goto waitForNewConn
			}
		}
	}

	return conn, nil
}

// Close implements the net.Listener interface
func (s *Server) Close() error {
	defer s.Echo.Close()
	defer s.TLSListener.Close()

	err := s.Echo.Close()
	if err != nil {
		return err
	}
	return s.TLSListener.Close()
}

// Addr implements the net.Listener interface
func (s *Server) Addr() net.Addr {
	return s.AddrStruct
}

// RegisterService adds a new service with it's associated math function
func (s *Server) RegisterService(handler Handler) {
	s.Handlers = append(s.Handlers, handler)
}

// DeregisterService removes a service base on the index
func (s *Server) DeregisterService(name string) {
	for i, service := range s.Handlers {
		if service.Name() == name {
			copy(s.Handlers[i:], s.Handlers[i+1:])
			s.Handlers[len(s.Handlers)-1] = nil // or the zero value of T
			s.Handlers = s.Handlers[:len(s.Handlers)-1]
		}
	}
}

// Dial is used to connect to on other server and set a prefix to access specific registered service
func (s *Server) Dial(addr, hostNamePrefix string, timeout time.Duration) (net.Conn, error) {
	hostName := s.getHostNameFromAddr(addr)

	if hostNamePrefix != "" {
		hostName = fmt.Sprintf("%s.%s", hostNamePrefix, hostName)
	}

	return NewServiceConnector(addr, hostName, s.Certificate, timeout)
}

// GetErrorChan returns a error channel which pipe error from the server
func (s *Server) GetErrorChan() chan error {
	return s.errChan
}
