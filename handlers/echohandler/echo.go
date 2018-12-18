package echohandler

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"regexp"

	"github.com/alexandrestein/securelink"
	"github.com/labstack/echo"
)

type (
	// Handler provides the interface for securelink.Handler interface with support
	// for Echo http framework
	Handler struct {
		*securelink.BaseHandler
		Echo       *echo.Echo
		httpServer *http.Server
		matchReg   *regexp.Regexp
	}
)

// New builds a new Handler with the given addr for the net.Listener interface,
// name which is used to deregister a service and the TLS configuration for Echo
func New(addr net.Addr, name string, tlsConfig *tls.Config) (*Handler, error) {
	rg, err := regexp.Compile(
		fmt.Sprintf("^%s\\.", name),
	)
	if err != nil {
		return nil, err
	}

	li := securelink.NewBaseListener(addr)

	httpServer := new(http.Server)
	httpServer.TLSConfig = tlsConfig
	httpServer.Addr = addr.String()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.TLSListener = li

	return &Handler{
		BaseHandler: &securelink.BaseHandler{
			NameField: name,
			Listener:  li,
		},
		Echo:       e,
		httpServer: httpServer,
		matchReg:   rg,
	}, nil
}

// Start needs to be called after all routes are registered
func (h *Handler) Start() error {
	return h.Echo.StartServer(h.httpServer)
}

// Handle provides the securelink.Handler interface
func (h *Handler) Handle(conn net.Conn) error {
	h.Listener.AcceptChan <- conn
	return nil
}

// Match implements the securelink.Handler
func (h *Handler) Match(serverName string) bool {
	return h.matchReg.MatchString(serverName)
}

func (h *Handler) Close() error {
	return h.Echo.Close()
}
