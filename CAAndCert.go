// Package securelink is not really for certificate management.
// It more a tool to make a cluster connection security easy.
// Build an save your CA. It will be able to generate Certificate pointers which
// can connect and check peer just on certificate validity.
//
// No need to check the host, you just want to make sur client and server use your CA.
package securelink

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"regexp"
	"time"
)

type (
	// Certificate provides an easy way to use certificates with tls package
	Certificate struct {
		Cert    *x509.Certificate
		KeyPair *KeyPair

		CACerts  []*x509.Certificate
		certPool *x509.CertPool
	}

	certExport struct {
		Cert    []byte
		KeyPair []byte
		CACerts [][]byte
	}

	// NewCertConfig is used to build a new certificate
	NewCertConfig struct {
		IsCA       bool
		IsWaldcard bool

		CertTemplate *x509.Certificate
		Parent       *Certificate

		LifeTime time.Duration

		PublicKey *KeyPair
	}
)

func buildCertPEM(input []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: input,
	})
}

// GetSignatureAlgorithm returns the signature algorithm for the given key type and key size
func GetSignatureAlgorithm(keyType KeyType, keyLength KeyLength) x509.SignatureAlgorithm {
	if keyType == KeyTypeRSA {
		switch keyLength {
		case KeyLengthRsa2048:
			return x509.SHA256WithRSAPSS
		case KeyLengthRsa3072:
			return x509.SHA384WithRSAPSS
		case KeyLengthRsa4096, KeyLengthRsa8192:
			return x509.SHA512WithRSAPSS
		}
	} else if keyType == KeyTypeEc {
		switch keyLength {
		case KeyLengthEc256:
			return x509.ECDSAWithSHA256
		case KeyLengthEc384:
			return x509.ECDSAWithSHA384
		case KeyLengthEc521:
			return x509.ECDSAWithSHA512
		}
	}
	return x509.UnknownSignatureAlgorithm
}

// NewCA returns a new CA pointer which is supposed to be used as server certificate
// and client and server certificate for remote instances.
// names are used as domain names.
func NewCA(config *NewCertConfig, names ...string) (*Certificate, error) {
	if config == nil {
		config = NewDefaultCertificationConfigWithDefaultTemplate(nil)
	}

	config.IsCA = true
	cert, err := newCert(config, names...)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

// ID returns the id as big.Int pointer
func (c *Certificate) ID() *big.Int {
	return c.Cert.SerialNumber
}

// NewCert returns a new certificate pointer which can be used for tls connection
func (c *Certificate) NewCert(config *NewCertConfig, names ...string) (*Certificate, error) {
	if !c.Cert.IsCA {
		return nil, fmt.Errorf("this is not a CA")
	}

	if config == nil {
		config = NewDefaultCertificationConfigWithDefaultTemplate(c)
	}

	config.Parent = c

	cert, err := newCert(config, names...)
	if err != nil {
		return nil, err
	}

	cert.CACerts = c.CACerts

	return cert, nil
}

func newCert(config *NewCertConfig, names ...string) (*Certificate, error) {
	if err := config.Valid(); err != nil {
		return nil, err
	}

	config.CertTemplate.IsCA = config.IsCA

	config.CertTemplate.DNSNames = append(config.CertTemplate.DNSNames, names...)
	config.wildcard()

	config.CertTemplate.NotBefore = time.Now()
	config.CertTemplate.NotAfter = time.Now().Add(config.LifeTime)

	config.CertTemplate.SignatureAlgorithm = GetSignatureAlgorithm(config.Parent.KeyPair.Type, config.Parent.KeyPair.Length)

	// Sign certificate with the CA
	certAsDER, err := x509.CreateCertificate(
		rand.Reader,
		config.CertTemplate,
		config.Parent.Cert,
		config.PublicKey.Public,
		config.Parent.KeyPair.Private,
	)
	if err != nil {
		return nil, err
	}

	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(certAsDER)
	if err != nil {
		return nil, err
	}

	caCerts := []*x509.Certificate{}
	config.Parent.CACerts = append(config.Parent.CACerts, config.Parent.Cert)
	for _, caCert := range config.Parent.CACerts {
		if caCert.Signature != nil || len(caCert.Signature) != 0 {
			caCerts = append(caCerts, caCert)
		}
	}

	return &Certificate{
		Cert:    cert,
		KeyPair: config.PublicKey,
		CACerts: caCerts,
	}, nil
}

// GetCertPEM is useful to start a new client or server with tls.X509KeyPair
func (c *Certificate) GetCertPEM() []byte {
	return buildCertPEM(c.Cert.Raw)
}

// GetTLSCertificate is useful in
// tls.Config{Certificates: []tls.Certificate{ca.GetTLSCertificate()}}
func (c *Certificate) GetTLSCertificate() tls.Certificate {
	cert, _ := tls.X509KeyPair(c.GetCertPEM(), c.KeyPair.GetPrivatePEM())
	return cert
}

// GetCertPool is useful in tls.Config{RootCAs: ca.GetCertPool()}
func (c *Certificate) GetCertPool() (pool *x509.CertPool) {
	if c.certPool != nil {
		return c.certPool
	}

	pool = x509.NewCertPool()

	for _, caCert := range c.CACerts {
		pool.AddCert(caCert)
	}

	if c.Cert.IsCA {
		pool.AddCert(c.Cert)
	}

	c.certPool = pool

	return
}

// Marshal convert the Certificate pointer into a slice of byte for
// transport or future use
func (c *Certificate) Marshal() ([]byte, error) {
	caCerts := [][]byte{}
	for _, caCert := range c.CACerts {
		if caCert.Raw == nil || len(caCert.Raw) == 0 {
			continue
		}
		caCerts = append(caCerts, caCert.Raw)
	}

	export := &certExport{
		Cert:    c.Cert.Raw,
		KeyPair: c.KeyPair.Marshal(),
		CACerts: caCerts,
	}

	return json.Marshal(export)
}

// Unmarshal build a new Certificate pointer with the information given
// by the input
func Unmarshal(input []byte) (*Certificate, error) {
	export := new(certExport)
	err := json.Unmarshal(input, export)
	if err != nil {
		return nil, err
	}

	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(export.Cert)
	if err != nil {
		return nil, err
	}

	var keyPair *KeyPair
	keyPair, err = UnmarshalKeyPair(export.KeyPair)
	if err != nil {
		return nil, err
	}

	caCerts := make([]*x509.Certificate, len(export.CACerts))
	for i, caCertAsBytes := range export.CACerts {
		var caCert *x509.Certificate
		caCert, err = x509.ParseCertificate(caCertAsBytes)
		if err != nil {
			return nil, err
		}

		caCerts[i] = caCert
	}

	return &Certificate{
		Cert:    cert,
		KeyPair: keyPair,
		CACerts: caCerts,
	}, nil
}

// NewDefaultCertificationConfig builds a new NewCertConfig pointer
// with the default values
func NewDefaultCertificationConfig(parent *Certificate) *NewCertConfig {
	return &NewCertConfig{
		IsCA:       false,
		IsWaldcard: true,

		LifeTime: DefaultCertLifeTime,

		Parent: parent,
	}
}

// NewDefaultCertificationConfigWithDefaultTemplate does the same ase above but
// with a default template
func NewDefaultCertificationConfigWithDefaultTemplate(parent *Certificate, names ...string) *NewCertConfig {
	ret := NewDefaultCertificationConfig(parent)
	ret.CertTemplate = GetCertTemplate(names, nil)
	return ret
}

// Valid checks if the caller has specified the minimum needed to
// have a valid certificate request
func (ncc *NewCertConfig) Valid() (err error) {
	if ncc.CertTemplate == nil {
		return fmt.Errorf("the template can't be empty")
	}

	if ncc.Parent == nil {
		err = ncc.genParent()
		if err != nil {
			return err
		}
	}

	return ncc.formatPublicKey()
}

func (ncc *NewCertConfig) genParent() error {
	keyPair, err := NewKeyPair(DefaultKeyType, DefaultKeyLength)
	if err != nil {
		return err
	}

	parent := new(Certificate)
	parent.Cert = ncc.CertTemplate
	parent.KeyPair = keyPair

	if ncc.PublicKey == nil {
		ncc.PublicKey = keyPair
	}

	ncc.IsCA = true
	ncc.IsWaldcard = true
	ncc.Parent = parent

	return nil
}

func (ncc *NewCertConfig) formatPublicKey() (err error) {
	if ncc.PublicKey == nil {
		ncc.PublicKey = new(KeyPair)
		ncc.PublicKey.Type = ncc.Parent.KeyPair.Type
		ncc.PublicKey.Length = ncc.Parent.KeyPair.Length
	} else if ncc.PublicKey.Public != nil {
		return nil
	} else {
		if ncc.PublicKey.Type == "" && ncc.PublicKey.Length == "" {
			ncc.PublicKey.Type = DefaultKeyType
			ncc.PublicKey.Length = DefaultKeyLength
		} else {
			ncc.formatPublicKeyPartlyDefined()
		}
	}

	ncc.PublicKey, err = NewKeyPair(ncc.PublicKey.Type, ncc.PublicKey.Length)
	return
}
func (ncc *NewCertConfig) formatPublicKeyPartlyDefined() {
	if ncc.PublicKey.Type != "" && ncc.PublicKey.Length == "" {
		switch ncc.PublicKey.Type {
		case KeyTypeEc:
			ncc.PublicKey.Length = DefaultKeyLength
		case KeyTypeRSA:
			ncc.PublicKey.Length = DefaultRSAKeyLength
		}
	} else if ncc.PublicKey.Type == "" && ncc.PublicKey.Length != "" {
		switch ncc.PublicKey.Length {
		case KeyLengthRsa2048, KeyLengthRsa3072, KeyLengthRsa4096, KeyLengthRsa8192:
			ncc.PublicKey.Type = KeyTypeRSA
		case KeyLengthEc256, KeyLengthEc384, KeyLengthEc521:
			ncc.PublicKey.Type = KeyTypeEc
		}
	}
}

func (ncc *NewCertConfig) wildcard() {
	if ncc.IsWaldcard {
		// Build a wildcard checker to not add a wildcard if already
		wildcardMatchRegexp := regexp.MustCompile(`^*\.`)

		// Range the given names
		for _, name := range ncc.CertTemplate.DNSNames {
			toAdd := ""
			// If not a wildcard
			if !wildcardMatchRegexp.MatchString(name) {
				toAdd = fmt.Sprintf("*.%s", name)
			} else {
				// This is already a wildcard domain, move to the next
				continue
			}

			// Check if the domain is not already present
			exist := false
			for _, name := range ncc.CertTemplate.DNSNames {
				if name == toAdd {
					exist = true
					break
				}
			}

			// If not found add the domain wildcard to the list
			if !exist {
				ncc.CertTemplate.DNSNames = append(ncc.CertTemplate.DNSNames, toAdd)
			}
		}
	}
}
