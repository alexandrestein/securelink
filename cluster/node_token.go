package cluster

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"time"

	base91 "github.com/Equim-chan/base91-go"
	"github.com/alexandrestein/securelink"
	"github.com/alexandrestein/securelink/common"
)

type (
	token struct {
		A *common.Addr
		C []byte
	}
)

// GetToken returns a string representation of a temporary token (10 minutes validity)
func (n *Node) GetToken() (string, error) {
	certConfig := securelink.NewDefaultCertificationConfigWithDefaultTemplate("TOKEN")
	certConfig.LifeTime = time.Minute * 5
	tmpCert, err := n.Certificate.NewCert(certConfig)
	if err != nil {
		return "", err
	}

	tokenObj := &token{
		A: n.AddrStruct,
		C: tmpCert.Marshal(),
	}

	var asJSON []byte
	asJSON, err = json.Marshal(tokenObj)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)

	_, err = zw.Write(asJSON)
	if err != nil {
		return "", err
	}

	err = zw.Close()
	if err != nil {
		return "", err
	}

	return base91.EncodeToString(buf.Bytes()), nil
}

// ReadToken returns values from the token.
// It gives the server address of the signer and the temporary certificate for connection.
// It returns error if any
func ReadToken(tokenString string) (addr *common.Addr, certificate *securelink.Certificate, err error) {
	compressed := base91.DecodeString(tokenString)

	buf := bytes.NewBuffer(compressed)
	var zr *gzip.Reader
	zr, err = gzip.NewReader(buf)
	if err != nil {

		return nil, nil, err
	}

	asJSON := make([]byte, 1000*20)

	var n int
	n, err = zr.Read(asJSON)
	if err != nil && err != io.EOF {
		return nil, nil, err
	}

	asJSON = asJSON[:n]

	err = zr.Close()
	if err != nil {
		return nil, nil, err
	}

	tokenObj := new(token)
	err = json.Unmarshal(asJSON, tokenObj)
	if err != nil {
		return nil, nil, err
	}

	certificate, err = securelink.Unmarshal(tokenObj.C)
	if err != nil {
		return nil, nil, err
	}

	return tokenObj.A, certificate, nil
}
