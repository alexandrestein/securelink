package securelink

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
)

type (
	// BaseHandler should be used as parent struct for custom services Handler
	BaseHandler struct {
		NameField      string
		Listener       *BaseListener
		HandleFunction FuncHandler
		MatchFunction  FuncServiceMatch
	}

	// BaseListener should be used as parent struct for custom services Listener
	BaseListener struct {
		AddrField  net.Addr
		AcceptChan chan net.Conn
	}

	// Handler provides a way to use multiple handlers inside a sign TLS listener.
	// You specify the TLS certificate for server but the same certificate is used in case
	// of Dial.
	Handler interface {
		Name() string

		Handle(conn net.Conn) error

		Match(hostName string) bool
	}

	// TransportConn is an interface to
	TransportConn struct {
		quic.Session
		Stream quic.Stream
		// Server bool
		Ctx context.Context

		cancel context.CancelFunc
	}
)

// NewHandler builds a new Hanlder pointer to use in a server object
func NewHandler(name string, serviceMatchFunc FuncServiceMatch, handlerFunction FuncHandler) Handler {
	return &BaseHandler{
		NameField:      name,
		HandleFunction: handlerFunction,
		MatchFunction:  serviceMatchFunc,
	}
}

// Handle is called when a client connect to the server and the client point to the service.
func (t *BaseHandler) Handle(conn net.Conn) (err error) {
	if t.HandleFunction == nil {
		return fmt.Errorf("no handler registered")
	}

	return t.HandleFunction(conn)
}

// Name returns the name of the handler.
// It is used manly when deregister is called.
//
// Implements Handler interface
func (t *BaseHandler) Name() string {
	return t.NameField
}

// Match returns true if the given hostname match the handler.
//
// Implements Handler interface
func (t *BaseHandler) Match(hostName string) bool {
	return t.MatchFunction(hostName)
}

func newTransportConn(session quic.Session, stream quic.Stream) (*TransportConn, error) {
	tc := &TransportConn{
		Session: session,
		Stream:  stream,
		// Server:  server,
	}

	return tc, nil
}

// GetID provides a way to get an ID which in the package can be found
// as the first host name from the certificate.
// This function contact the server at the given address with an "insecure" connection
// to get it's certificate. Checks that the certificate is valid for the given certificate if given.
// From the certificate it extract the first HostName which is return.
//
// In most case this function is called internally in the package.
func GetID(addr string, cert *Certificate) (serverID string) {
	tlsConfig := GetBaseTLSConfig("", cert)
	tlsConfig.InsecureSkipVerify = true

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	session, err := quic.DialAddrContext(ctx, addr, tlsConfig, nil)
	if err != nil {
		return ""
	}

	// conn, err := tls.Dial("tcp", string(addr), tlsConfig)
	// if err != nil {
	// 	return ""
	// }

	// err = conn.Handshake()
	// if err != nil {
	// 	return ""
	// }

	if len(session.ConnectionState().PeerCertificates) < 1 {
		return ""
	}

	remoteCert := session.ConnectionState().PeerCertificates[0]
	opts := x509.VerifyOptions{
		Roots: cert.CertPool,
	}

	if _, err := remoteCert.Verify(opts); err != nil {
		return ""
	}

	return remoteCert.SerialNumber.String()
}

// NewBaseListener returns a easy to extend struct pointer which can be used to
// register net.Listener interface in the package
func NewBaseListener(addr net.Addr) *BaseListener {
	return &BaseListener{
		AcceptChan: make(chan net.Conn),
		AddrField:  addr,
	}
}

// Accept implements the net.Listener interface
func (l *BaseListener) Accept() (net.Conn, error) {
	conn := <-l.AcceptChan
	return conn, nil
}

// Close implements the net.Listener interface
func (l *BaseListener) Close() error {
	return nil
}

// Addr implements the net.Listener interface
func (l *BaseListener) Addr() net.Addr {
	return l.AddrField
}

// func (t *TransportConn) Close() error {
// 	return t.Conn.Close()
// }

// func (t *TransportConn) LocalAddr() error {
// 	return t.Conn.Close()
// }

func (t *TransportConn) Read(b []byte) (int, error) {
	return t.Stream.Read(b)
}
func (t *TransportConn) Write(b []byte) (int, error) {
	return t.Stream.Write(b)
}

func (t *TransportConn) SetDeadline(dlT time.Time) error {
	return t.Stream.SetDeadline(dlT)
}
func (t *TransportConn) SetReadDeadline(dlT time.Time) error {
	return t.Stream.SetReadDeadline(dlT)
}
func (t *TransportConn) SetWriteDeadline(dlT time.Time) error {
	return t.Stream.SetWriteDeadline(dlT)
}

// func (t *TransportConn) Close() error {
// 	return t.Stream.Close()
// }
