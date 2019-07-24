package cluster

import (
	"net/http"

	"github.com/labstack/echo"
)

func (n *Node) initHandlers() {
	clusterGroup := n.Echo.Group(("/_cluster"))

	clusterGroup.GET("/join", joinHandler)
}

func joinHandler(c echo.Context) error {

	return c.NoContent(http.StatusNoContent)
}
