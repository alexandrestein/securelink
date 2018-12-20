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
	joinObj := &struct {
		HS      []byte
		Entries [][]byte
		Applied uint64
	}{}

	err := c.Bind(joinObj)
	if err != nil {
		return err
	}

	hs := raftpb.HardState{}
	// buff := make([]byte, 4096)
	// defer c.Request().Body.Close()
	// var n int
	// n, err = c.Request().Body.Read(buff)
	// if err != nil {
	// 	return err
	// }
	// buff = buff[:n]

	err = hs.Unmarshal(joinObj.HS)
	if err != nil {
		return err
	}

	err = h.Handler.Raft.Storage.SetHardState(hs)
	if err != nil {
		return err
	}

	h.Handler.Raft.applied = joinObj.Applied

	entries := make([]raftpb.Entry, len(joinObj.Entries))
	for i, entryAsBytes := range joinObj.Entries {
		err = entries[i].Unmarshal(entryAsBytes)
		if err != nil {
			return err
		}
	}

	err = h.Handler.Raft.Storage.Append(entries)
	if err != nil {
		return err
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*750)
	defer cancel()
	err = h.Raft.Node.Step(ctx, msg)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return nil
}
