package cluster

func (n *Node) initHandlers() {
	clusterGroup := n.Echo.Group(("/_cluster"))

	clusterGroup.GET("/", nil)
}
