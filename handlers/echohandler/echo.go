package echohandler

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"regexp"

	"github.com/alexandre/securelink"
	"github.com/labstack/echo"
)

type (
	Handler struct {
		*securelink.BaseHandler
		Echo       *echo.Echo
		httpServer *http.Server
		matchReg   *regexp.Regexp
	}

	// Listener struct {
	// 	addr       net.Addr
	// 	acceptChan chan net.Conn
	// }
)

func New(addr net.Addr, name string, tlsConfig *tls.Config) (*Handler, error) {
	rg, err := regexp.Compile(
		fmt.Sprintf("^%s\\.", name),
	)
	if err != nil {
		return nil, err
	}

	// li := &securelink.BaseListener{
	// 	acceptChan: make(chan net.Conn),
	// 	addr:       addr,
	// }
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
	// err := http2.ConfigureServer(h.httpServer, nil)
	// if err != nil {
	// 	return err
	// }

	// fmt.Println("sss", h.Echo.Server.)
	return h.Echo.StartServer(h.httpServer)
	// return h.Echo.StartServer(h.httpServer)
	// return h.Echo.Server.ServeTLS(h.Echo.TLSListener, "", "")
}

func (h *Handler) Handle(conn net.Conn) error {
	fmt.Println("handle")
	h.Listener.AcceptChan <- conn
	// fmt.Println("close")
	// conn.Close()
	return nil
}

func (h *Handler) Match(serverName string) bool {
	return h.matchReg.MatchString(serverName)
}

// func (h *Handler) Name() string {
// 	return h.Name
// }

// func (l *Listener) Accept() (net.Conn, error) {
// 	fmt.Println("accept")
// 	conn := <-l.acceptChan
// 	return conn, nil
// }
// func (l *Listener) Close() error {
// 	return nil
// }
// func (l *Listener) Addr() net.Addr {
// 	return l.addr
// }
