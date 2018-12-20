package echohandler

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/alexandrestein/securelink"
	"github.com/labstack/echo"
	"golang.org/x/net/http2"
)

type (
	// Handler provides the interface for securelink.Handler interface with support
	// for Echo http framework
	Handler struct {
		*securelink.BaseHandler
		Echo     *echo.Echo
		matchReg *regexp.Regexp
		http     *httpStruct
	}

	httpStruct struct {
		server          *http.Server
		h2Server        *http2.Server
		h2ServeConnOpts *http2.ServeConnOpts
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

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	httpServer := new(http.Server)
	httpServer.TLSConfig = tlsConfig
	httpServer.Addr = addr.String()
	httpServer.Handler = e

	http2.ConfigureServer(httpServer, nil)

	e.TLSServer = httpServer

	h2 := new(http2.Server)

	return &Handler{
		BaseHandler: &securelink.BaseHandler{
			NameField: name,
		},
		Echo: e,
		http: &httpStruct{
			server:   httpServer,
			h2Server: h2,
			h2ServeConnOpts: &http2.ServeConnOpts{
				BaseConfig: httpServer,
				Handler:    e,
			},
		},
		matchReg: rg,
	}, nil
}

// Start needs to be called after all routes are registered
func (h *Handler) Start() error {
	return h.Echo.StartServer(h.http.server)
}

// Handle provides the securelink.Handler interface
func (h *Handler) Handle(conn net.Conn) error {
	h.http.h2Server.ServeConn(conn, h.http.h2ServeConnOpts)
	return nil
}

// Match implements the securelink.Handler
func (h *Handler) Match(serverName string) bool {
	return h.matchReg.MatchString(serverName)
}

// Close closes Echo and other related servers
func (h *Handler) Close() error {
	h.Listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := h.Echo.Shutdown(ctx)
	if err != nil {
		return h.Echo.Close()
	}
	return nil
}
