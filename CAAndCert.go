// Package securelink tries to simplify the work need to mange and build certificates.
//
// This package can be used for PKI infrastructure, build a VPN like at the application level or
// simply to build certificates very quickly with no hassle.
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

		CACert   *x509.Certificate
		CertPool *x509.CertPool
		IsCA     bool
	}

	certExport struct {
		Cert    []byte
		KeyPair []byte
		CACert  []byte
	}

	// NewCertConfig is used to build a new certificate
	NewCertConfig struct {
		IsCA       bool
		IsWaldcard bool

		CertTemplate *x509.Certificate
		Parent       *Certificate

		LifeTime time.Duration

		KeyType   KeyType
		KeyLength KeyLength

		PublicKey *KeyPair

		CertPool *x509.CertPool
	}
)

// BuildCertPEM builds a PEM enceded x509 certificate
func BuildCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func genKeyPair(keyType KeyType, keyLength KeyLength) (*KeyPair, error) {
	if keyType == KeyTypeEd25519 {
		return NewEd25519(), nil
	} else if keyType == KeyTypeRSA {
		switch keyLength {
		case KeyLengthRsa2048, KeyLengthRsa3072, KeyLengthRsa4096, KeyLengthRsa8192:
			return NewRSA(keyLength), nil
		}
	} else if keyType == KeyTypeEc {
		switch keyLength {
		case KeyLengthEc256, KeyLengthEc384, KeyLengthEc521:
			return NewEc(keyLength), nil
		}
	}

	return nil, ErrKeyConfigNotCompatible
}

// GetSignatureAlgorithm returns the signature algorithm for the given key type and size
func GetSignatureAlgorithm(keyType KeyType, keyLength KeyLength) x509.SignatureAlgorithm {
	if keyType == KeyTypeEd25519 {
		return x509.PureEd25519
	} else if keyType == KeyTypeRSA {
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

// NewCA returns a new CA pointer.
// Names are used as domain names.
func NewCA(config *NewCertConfig, names ...string) (*Certificate, error) {
	config.IsCA = true

	cert, err := newCert(config, names...)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert.Cert)
	cert.CertPool = certPool

	return cert, nil
}

// ID returns the id of the certificate as big.Int pointer
func (c *Certificate) ID() *big.Int {
	return c.Cert.SerialNumber
}

// NewCert returns a new sign certificate with the given domains name.
// You can specify the options you want. Some options are part of the package like "wildcard" certificate,
// but others or based on the *x509.Certificate element inside *NewCertConfig.
func (c *Certificate) NewCert(config *NewCertConfig, names ...string) (*Certificate, error) {
	if !c.IsCA {
		return nil, fmt.Errorf("this is not a CA")
	}

	if config == nil {
		config = NewDefaultCertificationConfig()
	}

	config.Parent = c

	if config.CertPool == nil {
		config.CertPool = c.GetCertPool()
	} else {
		config.CertPool.AppendCertsFromPEM(c.GetCertPEM())
	}

	return newCert(config, names...)
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

	return &Certificate{
		Cert:     cert,
		KeyPair:  config.PublicKey,
		CACert:   config.Parent.Cert,
		CertPool: config.CertPool,
		IsCA:     config.IsCA,
	}, nil
}

// GetCertPEM is useful to start a new client or server with tls.X509KeyPair
func (c *Certificate) GetCertPEM() []byte {
	return BuildCertPEM(c.Cert)
}

// GetKeyPEM returns the key files, it is similar to *Certificate.GetCertPEM.
func (c *Certificate) GetKeyPEM() []byte {
	return c.KeyPair.GetPrivatePEM()
}

// GetTLSCertificate is useful in
// tls.Config{Certificates: []tls.Certificate{ca.GetTLSCertificate()}}
func (c *Certificate) GetTLSCertificate() tls.Certificate {
	cert, _ := tls.X509KeyPair(c.GetCertPEM(), c.KeyPair.GetPrivatePEM())
	return cert
}

// GetCertPool is useful in tls.Config{RootCAs: ca.GetCertPool()}
func (c *Certificate) GetCertPool() *x509.CertPool {
	return c.CertPool
}

// Marshal convert the Certificate pointer into a slice of byte for
// transport or future use
func (c *Certificate) Marshal() []byte {
	export := &certExport{
		Cert:    c.Cert.Raw,
		KeyPair: c.KeyPair.Marshal(),
		CACert:  c.CACert.Raw,
	}

	ret, _ := json.Marshal(export)

	return ret
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

	certPool := x509.NewCertPool()
	var caCert *x509.Certificate
	caCert, err = x509.ParseCertificate(export.CACert)
	if err != nil {
		return nil, err
	}
	certPool.AddCert(caCert)

	return &Certificate{
		Cert:     cert,
		KeyPair:  keyPair,
		CACert:   caCert,
		CertPool: certPool,
		IsCA:     cert.IsCA,
	}, nil
}

// NewDefaultCertificationConfig builds a new NewCertConfig pointer
// with the default values
func NewDefaultCertificationConfig() *NewCertConfig {
	return &NewCertConfig{
		IsCA:       false,
		IsWaldcard: true,

		LifeTime:  DefaultCertLifeTime,
		KeyType:   DefaultKeyType,
		KeyLength: DefaultKeyLength,

		CertTemplate: GetCertTemplate(nil, nil),
	}
}

// NewDefaultCertificationConfigWithDefaultTemplate does the same ase above but
// with a default template
func NewDefaultCertificationConfigWithDefaultTemplate(names ...string) *NewCertConfig {
	ret := NewDefaultCertificationConfig()
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
		if ncc.KeyType == "" || ncc.KeyLength == "" {
			ncc.KeyType = DefaultKeyType
			ncc.KeyLength = DefaultKeyLength
		}
		err = ncc.genParent()
		if err != nil {
			return err
		}
	}

	if ncc.CertPool == nil {
		certPool := x509.NewCertPool()
		certPool.AddCert(ncc.Parent.Cert)
		ncc.CertPool = certPool
	}

	if ncc.PublicKey == nil {
		ncc.PublicKey, err = NewKeyPair(ncc.KeyType, ncc.KeyLength)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ncc *NewCertConfig) genParent() error {
	// nccCp := new(NewCertConfig)
	keyPair, err := genKeyPair(ncc.KeyType, ncc.KeyLength)
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

// func (ncc *NewCertConfig) genPublicKey() (err error) {
// 	ncc.PublicKey, err = genKeyPair(ncc.KeyType, ncc.KeyLength)
// 	return
// }

func (ncc *NewCertConfig) wildcard() {
	if ncc.IsWaldcard {
		// Build a wildcard checker to not add a wildcard if already
		wildcardMatchRegexp := regexp.MustCompile("^*\\.")

		// If a global wildcard is set no need to add some other
		// subname wildcard
		for _, name := range ncc.CertTemplate.DNSNames {
			if name == "*" {
				return
			}
		}

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
