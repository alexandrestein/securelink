package securelink

import (
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

	sortPeersBy(sortPeersByPriority).Sort(mp.Peers)

	n.lock.Lock()
	n.clusterMap = mp
	n.lock.Unlock()

	return nil
}

func (n *Node) pingHandler(c echo.Context) error {
	n.lock.RLock()
	defer n.lock.RUnlock()

	pStruct := new(ping)
	err := c.Bind(pStruct)
	if err != nil {
		return err
	}

	if pStruct.Time.After(n.clusterMap.Update) {
		n.Server.Logger.Infof("*Node.pingHandler: the local node %s:%s is late", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String())
		go n.getUpdate(pStruct.Master)
	}

	prStruct := &ping{
		Time:   n.clusterMap.Update,
		Master: n.getMaster().ID,
	}

	return c.JSON(http.StatusOK, prStruct)
}

func (n *Node) updateHandler(c echo.Context) error {
	n.lock.RLock()
	defer n.lock.RUnlock()
	return c.JSON(http.StatusOK, n.clusterMap)
}

func (n *Node) failureHandler(c echo.Context) error {
	master := n.getMaster()

	failedNode := new(Peer)
	err := c.Bind(failedNode)
	if err != nil {
		return err
	}

	n.Server.Logger.Infof("got signal %s:%s is down", failedNode.ID.String(), failedNode.Addr.String())

	failover := false

	// Check if the local node is master
	if master.ID.Uint64() != n.LocalConfig.ID.Uint64() {
		// It is not master but the pointing failed node is the master
		if master.ID.Uint64() == failedNode.ID.Uint64() {
			// Get the master failover
			masterFailover := n.getMasterFailover()
			// Check if local node is the failover node
			if masterFailover.ID.Uint64() == n.LocalConfig.ID.Uint64() {
				failover = true
				// master = masterFailover
				goto doIt
			}
		}
		// local node is not master and the pointed node is not master
		n.Server.Logger.Errorf("got down signal but local node is note master")
		return c.String(http.StatusBadGateway, "not master")
	}

doIt:
	failed := !n.checkPeerAlive(failedNode)

	if failed {

		err = n.togglePeer(failedNode.ID, failover, true)
		if err != nil {
			if err.Error() == "already down" {
				return c.NoContent(http.StatusNoContent)
			}
			n.Server.Logger.Errorf("fail to toggle peer: %s", err.Error())
			return c.String(http.StatusInternalServerError, err.Error())
		}

		n.Server.Logger.Infof("confirmation %s:%s is down", failedNode.ID.String(), failedNode.Addr.String())
		n.Server.Logger.Warningf("local %s:%s takes the lead", n.LocalConfig.ID.String(), n.LocalConfig.Addr.String())

		return c.NoContent(http.StatusNoContent)
	}

	n.Server.Logger.Infof("%s:%s is not down", failedNode.ID.String(), failedNode.Addr.String())
	return c.String(http.StatusBadRequest, "server replied")
}
