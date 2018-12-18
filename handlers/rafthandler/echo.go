package rafthandler

func (h *Handler) initEcho() error {
	e := h.Transport.EchoHandler.Echo

	hh := &httpHandler{h}

	// e.GET("/", hh.GetServerInfo)
	e.POST(AddNode, hh.AddNode)
	e.HEAD(StartNodes, hh.Start)
	e.POST(Message, hh.Message)

	return nil
}
