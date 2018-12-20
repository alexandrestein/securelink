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
	// DefaultRequestTimeOut is used when no timeout are specified
	DefaultRequestTimeOut = time.Second * 5
)

// Dial start a new HTTP client to communicate with other nodes
func (t *Transport) Dial(destID uint64, timeout time.Duration) (*http.Client, *Peer, error) {
	if timeout == 0 {
		timeout = DefaultRequestTimeOut
	}

	for _, peer := range t.Peers.peers {
		if peer.ID == destID {
			// To know if the peer has been updated
			updated := false
			var cli *http.Client
			// If the cli is nil or the deadline is exceeded
			if peer.cli == nil || time.Now().After(peer.cliDeadline) {
				addr := fmt.Sprintf("%s.%d", HostPrefix, peer.ID)
				cli = securelink.NewHTTPSConnector(addr, t.TLS.Certificate)
				cli.Timeout = timeout

				// Lock the peers for any concurrent corruption
				updated = true
				t.Peers.lock.Lock()
				peer.cli = cli
				peer.cliDeadline = time.Now().Add(timeout)
			} else {
				// Cli exist and the deadline is not exceeded it returns
				// the save cli
				cli = peer.cli

				// It checks if the deadline is not almost passed.
				// If the deadline is almost passed the (most of the time is gone)
				// the deadline is updated to the given one.
				//
				// this is not done for every call to limit the numbers of lock.
				// Every peers are lock during this process.
				if time.Now().After(peer.cliDeadline.Add(-timeout / 2)) {
					updated = true
					t.Peers.lock.Lock()
					peer.cliDeadline = time.Now().Add(timeout)
				}
			}

			if updated {
				t.Peers.lock.Unlock()
			}

			return cli, peer, nil
		}
	}

	return nil, nil, fmt.Errorf("ID not found")
}

// Get is a simplifier which use Dial function to return the GET response for the given
// peer ID and the given URL
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

// GetBytes does same as above but instead of returning a http.Response pointer it returns directly
// the content of the body
func (t *Transport) GetBytes(destID uint64, url string, timeout time.Duration) ([]byte, error) {
	resp, err := t.Get(destID, url, timeout)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

// PostJSON is a simplifier which use Dial function to send the given content for the given
// peer ID and the given URL and return the POST response
func (t *Transport) PostJSON(destID uint64, url string, content []byte, timeout time.Duration) (*http.Response, error) {
	cli, peer, err := t.Dial(destID, timeout)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewBuffer(content)
	var resp *http.Response
	resp, err = cli.Post(peer.BuildURL(url), echo.MIMEApplicationJSON, reader)
	if err != nil {
		return nil, err
	} else if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		return nil, ErrBadResponseCode(resp.StatusCode)
	}
	return resp, err
}

// PostJSONToAll does same as above but for all registered peers
func (t *Transport) PostJSONToAll(url string, content []byte, timeout time.Duration) error {
	errChan := make(chan error, t.Peers.Len())
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
				errChan <- err
			}
		}(peer)
	}

	wg.Wait()

	return t.manageErrorForMultipleRequests(errChan)
}

// HeadToAll try to contact every peers to send them a HEAD request
func (t *Transport) HeadToAll(url string, timeout time.Duration) error {
	errChan := make(chan error, t.Peers.Len())
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
				errChan <- err
			}

			var resp *http.Response
			resp, err = cli.Head(peer.BuildURL(url))
			if err != nil {
				errChan <- err
			} else if resp.StatusCode < 200 || 300 <= resp.StatusCode {
				errChan <- ErrBadResponseCode(resp.StatusCode)
			}

		}(peer)
	}

	wg.Wait()

	return t.manageErrorForMultipleRequests(errChan)
}

func (t *Transport) manageErrorForMultipleRequests(errChan chan error) error {
	var globalErr *multierror.Error
notDone:
	select {
	case err := <-errChan:
		globalErr = multierror.Append(globalErr, err)
		goto notDone
	default:
	}

	return globalErr.ErrorOrNil()
}

// SendMessageTo sends the given message content to given node
func (t *Transport) SendMessageTo(destID uint64, message []byte, timeout time.Duration) error {
	resp, err := t.PostJSON(destID, Message, message, timeout)
	if err != nil {
		return err
	} else if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		return ErrBadResponseCode(resp.StatusCode)
	}
	return err
}
