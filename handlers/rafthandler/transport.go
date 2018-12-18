package rafthandler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/labstack/echo"

	"github.com/alexandrestein/securelink"
)

var (
	// DefaultRequestTimeOut is used when no timeout are
	// specified
	DefaultRequestTimeOut = time.Second
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
	if timeout == 0 {
		timeout = DefaultRequestTimeOut
	}
	for _, peer := range t.Peers.peers {
		if peer.ID == destID {
			addr := fmt.Sprintf("%s.%d", HostPrefix, peer.ID)
			cli := securelink.NewHTTPSConnector(addr, t.TLS.Certificate)
			cli.Timeout = timeout
			return cli, peer, nil
		}
	}

	return nil, nil, fmt.Errorf("ID not found")
}

func (t *Transport) Get(destID uint64, url string, timeout time.Duration) (*http.Response, error) {
	cli, peer, err := t.Dial(destID, timeout)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	resp, err = cli.Get(peer.BuildURL(url))
	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		return nil, ErrBadResponseCode(resp.StatusCode)
	}
	return resp, nil
}

func (t *Transport) GetBytes(destID uint64, url string, timeout time.Duration) ([]byte, error) {
	resp, err := t.Get(destID, url, timeout)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (t *Transport) PostJSON(destID uint64, url string, content []byte, timeout time.Duration) (*http.Response, error) {
	cli, peer, err := t.Dial(destID, timeout)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewBuffer(content)
	var resp *http.Response
	resp, err = cli.Post(peer.BuildURL(url), echo.MIMEApplicationJSON, reader)
	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		return nil, ErrBadResponseCode(resp.StatusCode)
	}
	return resp, err
}

func (t *Transport) HeadToAll(url string, timeout time.Duration) error {
	var globalErr *multierror.Error
	var wg sync.WaitGroup
	for _, peer := range t.Peers.peers {
		if peer.ID == t.ID().Uint64() {
			continue
		}

		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()
			cli, _, err := t.Dial(peer.ID, timeout)
			if err != nil {
				globalErr = multierror.Append(globalErr, err)
			}

			var resp *http.Response
			resp, err = cli.Head(peer.BuildURL(url))
			if err != nil {
				globalErr = multierror.Append(globalErr, err)
			} else if resp.StatusCode < 200 || 300 <= resp.StatusCode {
				globalErr = multierror.Append(globalErr, ErrBadResponseCode(resp.StatusCode))
			}
		}(peer)
	}

	wg.Wait()
	return globalErr.ErrorOrNil()
}

func (t *Transport) PostJSONToAll(url string, content []byte, timeout time.Duration) error {
	var globalErr *multierror.Error
	var wg sync.WaitGroup
	for _, peer := range t.Peers.peers {
		if peer.ID == t.ID().Uint64() {
			continue
		}

		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()
			_, err := t.PostJSON(peer.ID, url, content, timeout)
			if err != nil {
				globalErr = multierror.Append(globalErr, err)
			}
		}(peer)
	}

	wg.Wait()
	return globalErr.ErrorOrNil()
}

func (t *Transport) SendMessageTo(destID uint64, message []byte, timeout time.Duration) error {
	resp, err := t.PostJSON(destID, Message, message, timeout)
	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		return ErrBadResponseCode(resp.StatusCode)
	}
	return err
}
