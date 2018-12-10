package rafthandler

import (
	"context"
	"fmt"
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
	fmt.Println("get from server")

	peer := new(Peer)
	err := c.Bind(peer)
	// err = json.Unmarshal(peerAsBytes, peer)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	err = h.Raft.addNode(peer)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return nil
}

func (h *httpHandler) Message(c echo.Context) error {
	messageAsBytes, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	msg := raftpb.Message{}
	err = msg.Unmarshal(messageAsBytes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*750)
	defer cancel()
	err = h.Raft.Node.Step(ctx, msg)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return nil
}
