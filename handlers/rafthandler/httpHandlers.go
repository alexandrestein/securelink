package rafthandler

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/labstack/echo"
)

type (
	httpHandler struct {
		*Handler
	}
)

func (h *httpHandler) GetServerInfo(c echo.Context) error {
	return nil
}

func (h *httpHandler) AddNode(c echo.Context) error {
	peers := []*Peer{}
	err := c.Bind(&peers)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	err = h.Raft.addNode(peers...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return nil
}

func (h *httpHandler) Start(c echo.Context) error {
	return h.Handler.Raft.Start(false)
}

func (h *httpHandler) Join(c echo.Context) error {
	return h.Handler.Raft.Start(true)
}

func (h *httpHandler) Message(c echo.Context) error {
	if h.Raft.Node == nil {
		return ErrRaftNodeNotLoaded
	}

	messageAsBytes, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	msg := raftpb.Message{}
	err = msg.Unmarshal(messageAsBytes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	if h.Raft.Node == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*750)
	defer cancel()
	err = h.Raft.Node.Step(ctx, msg)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return nil
}

// func (h *httpHandler)  RMNode(c echo.Context) error {
// 	h.

// 	return nil
// }
