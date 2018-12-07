package rafthandler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/labstack/echo"

	"github.com/alexandrestein/securelink"
)

// func (t *Transport) Accept() (net.Conn, error) {
// 	fmt.Println("accept")
// 	conn, ok := <-t.AcceptChan
// 	if !ok {
// 		return nil, fmt.Errorf("looks close")
// 	}
// 	return conn, nil
// }

// func (t *Transport) Addr() net.Addr {
// 	return t.Server.Addr()
// }

// func (t *Transport) Close() error {
// 	return t.Server.Close()
// }

func (t *Transport) Dial(destID uint64, timeout time.Duration) (*http.Client, *Peer, error) {
	for _, peer := range t.Peers.Peers {
		if peer.ID == destID {
			// return t.Server.Dial(peer.String(), HostPrefix, timeout)
			addr := fmt.Sprintf("%s.%d", HostPrefix, peer.ID)
			fmt.Println("addr", addr)
			return securelink.NewHTTPSConnector(addr, t.TLS.Certificate), peer, nil
		}
	}

	return nil, nil, fmt.Errorf("ID not found")
}

func (t *Transport) Get(destID uint64, url string) (*http.Response, error) {
	cli, peer, err := t.Dial(destID, time.Second)
	if err != nil {
		return nil, err
	}

	return cli.Get(peer.BuildURL(url))
}

func (t *Transport) GetBytes(destID uint64, url string) ([]byte, error) {
	resp, err := t.Get(destID, url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (t *Transport) PostJSON(destID uint64, url string, content []byte) (*http.Response, error) {
	cli, peer, err := t.Dial(destID, time.Second)
	if err != nil {
		return nil, err
	}

	// reader := bytes.NewBuffer(content)
	// reader := bytes.NewReader(content)
	fmt.Println("dialed", peer.BuildURL(url))
	var ret *http.Response
	ret, err = cli.Post(peer.BuildURL(url), echo.MIMEApplicationJSON, nil)
	fmt.Println("dialed bis", ret, err)
	return ret, err
}

func (t *Transport) PostJSONToAll(url string, content []byte) error {
	var globalErr error
	var wg sync.WaitGroup
	for _, peer := range t.Peers.Peers {
		fmt.Println("add", peer)
		if peer.ID == t.ID().Uint64() {
			continue
		}

		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()
			defer fmt.Println("done")
			fmt.Println("post")
			_, err := t.PostJSON(peer.ID, url, content)
			fmt.Println("posted")
			multierror.Append(globalErr, err)
		}(peer)
	}

	fmt.Println("wait")

	wg.Wait()
	fmt.Println("wait done")
	return globalErr
}

func (t *Transport) SendMessageTo(destID uint64, message []byte) error {
	_, err := t.PostJSON(destID, Message, message)
	return err
}
