package rafthandler

func (h *Handler) initEcho() error {
	e := h.Transport.EchoHandler.Echo

	hh := &httpHandler{h}

	e.GET("/", hh.GetServerInfo)
	e.POST("/addNode", hh.AddNode)
	e.POST("/message", hh.Message)

	return nil
}
