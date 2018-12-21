package rafthandler

func (h *Handler) initEcho() error {
	e := h.Transport.EchoHandler.Echo

	hh := &httpHandler{h}

	// e.GET("/", hh.GetServerInfo)
	e.POST(AddNode, hh.AddNode)
	// e.POST(RMNode, hh.RmNode)
	e.HEAD(StartNodes, hh.Start)
	e.HEAD(JoinCluster, hh.Join)
	e.POST(Message, hh.Message)

	return nil
}
