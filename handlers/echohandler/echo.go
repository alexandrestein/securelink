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
	Handler struct {
		*securelink.BaseHandler
		Echo       *echo.Echo
		httpServer *http.Server
		matchReg   *regexp.Regexp
	}
)

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

func (h *Handler) Start() error {
	return h.Echo.StartServer(h.httpServer)
}

func (h *Handler) Handle(conn net.Conn) error {
	h.Listener.AcceptChan <- conn

	// time.Sleep(time.Second * 10)
	// conn.Close()

	return nil
}

func (h *Handler) Match(serverName string) bool {
	return h.matchReg.MatchString(serverName)
}
