package securelink

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"
)

// Defines the supported key type
const (
	KeyTypeEd25519 KeyType = "ed25519 Elliptic Curve"
	KeyTypeRSA     KeyType = "RSA"
	KeyTypeEc      KeyType = "NIST Elliptic Curve"
)

// Defines the supported key length
const (
	KeyLengthEd25519 KeyLength = "EC 25519"

	KeyLengthRsa2048 KeyLength = "RSA 2048"
	KeyLengthRsa3072 KeyLength = "RSA 3072"
	KeyLengthRsa4096 KeyLength = "RSA 4096"
	KeyLengthRsa8192 KeyLength = "RSA 8192"

	KeyLengthEc256 KeyLength = "EC 256"
	KeyLengthEc384 KeyLength = "EC 384"
	KeyLengthEc521 KeyLength = "EC 521"
)

// Defaults values for NewCertConfig
var (
	DefaultCertLifeTime = time.Hour * 24 * 30 * 3 // 3 months
	// DefaultKeyType      = KeyTypeEd25519
	// DefaultKeyLength    = KeyLengthEd25519
	DefaultKeyType   = KeyTypeEc
	DefaultKeyLength = KeyLengthEc256
)

// Those variables defines the most common package errors
var (
	ErrKeyConfigNotCompatible = fmt.Errorf("the key type and key size are not compatible")
)

// GetCertTemplate returns the base template for certification.
// What is does:
// - it generates an random ID
// - sets the given names into the common names as wildcards
// - adds ips into the IP field if given
// - sets the certificate not before now and good for 1 year
// - gives almost every permissions to the certificate
// - sets the certificate as is not a CA
func GetCertTemplate(names []string, ips []net.IP) *x509.Certificate {
	serial, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))

	if len(names) == 0 || names == nil {
		names = []string{serial.String(), "*"}
	} else {
		names = append(names, serial.String())
	}

	return &x509.Certificate{
		SignatureAlgorithm: x509.UnknownSignatureAlgorithm,

		SerialNumber: serial,
		Subject:      getSubject(names[0]),

		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365), // Validity bounds.
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment | x509.KeyUsageKeyAgreement | x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageEncipherOnly | x509.KeyUsageDecipherOnly,

		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}, // Sequence of extended key usages.

		// BasicConstraintsValid indicates whether IsCA, MaxPathLen,
		// and MaxPathLenZero are valid.
		BasicConstraintsValid: true,
		IsCA:                  false,

		// MaxPathLen and MaxPathLenZero indicate the presence and
		// value of the BasicConstraints' "pathLenConstraint".
		//
		// When parsing a certificate, a positive non-zero MaxPathLen
		// means that the field was specified, -1 means it was unset,
		// and MaxPathLenZero being true mean that the field was
		// explicitly set to zero. The case of MaxPathLen==0 with MaxPathLenZero==false
		// should be treated equivalent to -1 (unset).
		//
		// When generating a certificate, an unset pathLenConstraint
		// can be requested with either MaxPathLen == -1 or using the
		// zero value for both MaxPathLen and MaxPathLenZero.
		MaxPathLen: -1,
		// MaxPathLenZero indicates that BasicConstraintsValid==true
		// and MaxPathLen==0 should be interpreted as an actual
		// maximum path length of zero. Otherwise, that combination is
		// interpreted as MaxPathLen not being set.
		MaxPathLenZero: false, // Go 1.4
		DNSNames:       names,
		IPAddresses:    ips, // Go 1.1
	}
}

func getSubject(sub string) pkix.Name {
	if sub == "" {
		sub = "secure-link"
	}

	return pkix.Name{
		CommonName: sub,
	}
}
