// Package securelink enable caller to stream multiple connection inside a sign
// TLS link. The package provide CA and certificate generation to have easy management.
package securelink

import (
	"crypto/tls"
	"fmt"
	"math/big"
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
		AddrStruct *common.Addr
		Services   *Services
		TLS        *TLS

		getHostNameFromAddr FuncGetHostNameFromAddr
		errChan             chan error
	}

	// TLS defines sub struct of Server which take care of TLS elements
	TLS struct {
		Listener    net.Listener
		Certificate *Certificate
		Config      *tls.Config
	}

	// Services is related to the handling of the connection
	Services struct {
		Echo *echo.Echo
		// httpServer *http.Server
		Handlers []Handler
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

	// tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2", "http/1.1")
	// tlsConfig.NextProtos = append(tlsConfig.NextProtos, "http/1.1", "h2")
	// tlsConfig.NextProtos = append(tlsConfig.NextProtos, "http/1.1")

	fmt.Println("tlsConfig 1", tlsConfig.NextProtos)
	var tlsListener net.Listener
	tlsListener, err = tls.Listen("tcp", addr.ForListenerBroadcast(), tlsConfig)
	if err != nil {
		return nil, err
	}
	fmt.Println("tlsConfig 2", tlsConfig.NextProtos)

	if getHostNameFromAddr == nil {
		getHostNameFromAddr = func(s string) string {
			return GetID(s, cert)
		}
	}

	s := &Server{
		Services: &Services{
			Echo:     echo.New(),
			Handlers: []Handler{},
		},
		TLS: &TLS{
			Listener:    tlsListener,
			Certificate: cert,
			Config:      tlsConfig,
		},

		AddrStruct:          addr,
		getHostNameFromAddr: getHostNameFromAddr,
		errChan:             make(chan error),
	}

	s.Services.Echo.TLSListener = s
	s.Services.Echo.HideBanner = true
	s.Services.Echo.HidePort = true

	httpServer := new(http.Server)
	httpServer.TLSConfig = tlsConfig
	httpServer.Addr = addr.String()
	httpServer.Handler = s.Services.Echo

	go func(e *Server, httpServer *http.Server) {
		// s.errChan <- httpServer.ServeTLS(s.Services.Echo.TLSListener, "", "")
		s.errChan <- s.Services.Echo.StartServer(httpServer)
		// s.errChan <- s.Services.Echo.StartTLS
	}(s, httpServer)

	return s, nil
}

// ID returns an id as big.Int pointer
func (s *Server) ID() *big.Int {
	return s.TLS.Certificate.ID()
}

// Accept implements the net.Listener interface
func (s *Server) Accept() (net.Conn, error) {
waitForNewConn:
	conn, err := s.TLS.Listener.Accept()
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
		for _, service := range s.Services.Handlers {
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
	defer s.Services.Echo.Close()
	defer s.TLS.Listener.Close()

	err := s.Services.Echo.Close()
	if err != nil {
		return err
	}
	return s.TLS.Listener.Close()
}

// Addr implements the net.Listener interface
func (s *Server) Addr() net.Addr {
	return s.AddrStruct
}

// RegisterService adds a new service with it's associated math function
func (s *Server) RegisterService(handler Handler) {
	s.Services.Handlers = append(s.Services.Handlers, handler)
}

// DeregisterService removes a service base on the index
func (s *Server) DeregisterService(name string) {
	for i, service := range s.Services.Handlers {
		if service.Name() == name {
			copy(s.Services.Handlers[i:], s.Services.Handlers[i+1:])
			s.Services.Handlers[len(s.Services.Handlers)-1] = nil // or the zero value of T
			s.Services.Handlers = s.Services.Handlers[:len(s.Services.Handlers)-1]
		}
	}
}

// Dial is used to connect to on other server and set a prefix to access specific registered service
func (s *Server) Dial(addr, hostNamePrefix string, timeout time.Duration) (net.Conn, error) {
	hostName := s.getHostNameFromAddr(addr)

	if hostNamePrefix != "" {
		hostName = fmt.Sprintf("%s.%s", hostNamePrefix, hostName)
	}

	return NewServiceConnector(addr, hostName, s.TLS.Certificate, timeout)
}

// GetErrorChan returns a error channel which pipe error from the server
func (s *Server) GetErrorChan() chan error {
	return s.errChan
}
