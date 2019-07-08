package replication

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/alexandrestein/gotinydb/securelink"
)

type (
	handler struct {
		*Node
	}
)

// Defines constants to for the API path
var (
	APIVersion = "v0.0.0"

	PostCertificatePATH = "new-client"

	ServerIDHeadearName = "Server-Id"
)

func (n *Node) settupHandlers() {
	h := &handler{n}

	h.Echo.Use(middleware.Recover())

	apiGroup := h.Echo.Group(
		fmt.Sprintf("/%s/", APIVersion),
		h.verifyCertificateMiddleware,
	)

	apiGroup.POST(PostCertificatePATH, h.returnCert)
	// apiGroup.GET(GetRaftStreamerPATH, h.raftStream)
}

func (h *handler) returnCert(c echo.Context) error {
	tokenAsString := c.FormValue("token")

	if !h.Certificate.VerifyToken(tokenAsString) {
		return fmt.Errorf("the given token is not valid")
	}

	if !h.Certificate.IsCA {
		return fmt.Errorf("the server is not a CA")
	}

	clientCert, err := h.Certificate.NewCert(DefaultCertKeyAlgorithm, DefaultCertKeyLength, time.Hour*24*365*10, securelink.GetCertTemplate(nil, nil))
	if err != nil {
		return err
	}
	clientCertAsBytes := clientCert.Marshal()

	// savedPeer := &securecache.SavedPeer{
	// 	Addrs: []string
	// }
	// securecache.PeersTable.Add(clientCert.Cert.SerialNumber, time.Hour*24*365*10, )

	return c.JSONBlob(http.StatusOK, clientCertAsBytes)
}

// func (h *handler) raftStream(c echo.Context) error {
// 	websocket.Handler(func(ws *websocket.Conn) {
// 		h.raftTransport.acceptChan <- ws
// 	}).ServeHTTP(c.Response(), c.Request())
// 	return nil
// }

// func (h *handler) serverConnectivity(c echo.Context) error {
// 	addrs, err := common.GetAddresses()
// 	if err != nil {
// 		return err
// 	}

// 	ret := &struct {
// 		Addrs []string
// 		Port  string
// 	}{
// 		addrs, h.Port,
// 	}

// 	return c.JSON(http.StatusOK, ret)
// }

func (h *handler) verifyCertificateMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return echo.HandlerFunc(func(c echo.Context) error {

		if !c.IsTLS() {
			return echo.ErrUnauthorized
		}

		if len(c.Request().TLS.PeerCertificates) <= 0 && c.Path() != fmt.Sprintf("/%s/%s", APIVersion, PostCertificatePATH) {
			return echo.NewHTTPError(http.StatusUnauthorized, "no client certificate")
		}

		c.Response().Header().Add(ServerIDHeadearName, h.GetID().String())

		return next(c)
	})
}
