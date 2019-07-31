package securelink

import (
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/labstack/echo"
)

type (
	ping struct {
		Time   time.Time
		Master *big.Int
	}
)

func (n *Node) joinHandler(c echo.Context) error {
	mp := &clusterMap{}
	err := c.Bind(mp)
	if err != nil {
		return err
	}

	n.lock.Lock()
	n.clusterMap = mp
	n.lock.Unlock()

	return nil
}

func (n *Node) pingHandler(c echo.Context) error {
	pStruct := new(ping)
	err := c.Bind(pStruct)
	if err != nil {
		return err
	}

	if pStruct.Time.After(n.clusterMap.Update) {
		fmt.Println("the local node is late", n.LocalConfig.ID)
		go n.getUpdate(pStruct.Master)
	}

	prStruct := &ping{
		Time:   n.clusterMap.Update,
		Master: n.getMaster().ID,
	}

	return c.JSON(http.StatusOK, prStruct)
}

func (n *Node) updateHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, n.clusterMap)
}

func (n *Node) failureHandler(c echo.Context) error {
	failedNode := new(Peer)

	err := c.Bind(failedNode)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, n.clusterMap)
}
