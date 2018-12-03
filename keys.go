package securelink

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

type (
	// KeyType is a simple string type to know which type of key it is about
	KeyType string
	// KeyLength is a simple string type to know which key size it is about
	KeyLength string

	// KeyPair defines a struct to manage different type and size of keys interopeably
	KeyPair struct {
		Type            KeyType
		Length          KeyLength
		Private, Public interface{}
	}

	keyPairExport struct {
		*KeyPair
		Private []byte `json:",omitempty"`
		Public  []byte `json:",omitempty"`
	}
)

// NewKeyPair builds a new key pair with the given options
func NewKeyPair(keyType KeyType, keyLength KeyLength) (*KeyPair, error) {
	switch keyType {
	case KeyTypeEc:
		return NewEc(keyLength), nil
	case KeyTypeRSA:
		return NewRSA(keyLength), nil
	default:
		return nil, fmt.Errorf("the given type is not valid: %q", string(keyType))
	}
}

func newKeyPair(keyType KeyType, keyLength KeyLength) *KeyPair {
	ret := new(KeyPair)
	ret.Type = keyType
	ret.Length = keyLength
	return ret
}

// NewRSA returns a new RSA key pair of the given size
func NewRSA(keyLength KeyLength) *KeyPair {
	length := 0
	switch keyLength {
	case KeyLengthRsa2048:
		length = 2048
	case KeyLengthRsa3072:
		length = 3072
	case KeyLengthRsa4096:
		length = 4096
	case KeyLengthRsa8192:
		length = 8192
	}

	ret := newKeyPair(KeyTypeRSA, keyLength)
	privateKey, _ := rsa.GenerateKey(rand.Reader, length)

	ret.Private = privateKey
	ret.Public = privateKey.Public()

	return ret
}

// NewEc returns a new "elliptic curve" key pair of the given size
func NewEc(keyLength KeyLength) *KeyPair {
	var curve elliptic.Curve
	switch keyLength {
	case KeyLengthEc256:
		curve = elliptic.P256()
	case KeyLengthEc384:
		curve = elliptic.P384()
	case KeyLengthEc521:
		curve = elliptic.P521()
	}

	ret := newKeyPair(KeyTypeEc, keyLength)

	privateKey, _ := ecdsa.GenerateKey(curve, rand.Reader)
	ret.Private = privateKey
	ret.Public = privateKey.Public()

	return ret
}

// GetPrivateDER returns a slice of bytes which represent the private key as DER encoded
func (k *KeyPair) GetPrivateDER() []byte {
	var ret []byte
	if k.Type == KeyTypeRSA {
		ret = x509.MarshalPKCS1PrivateKey(k.Private.(*rsa.PrivateKey))
	} else if k.Type == KeyTypeEc {
		ret, _ = x509.MarshalECPrivateKey(k.Private.(*ecdsa.PrivateKey))
	}
	return ret
}

// GetPrivatePEM returns a slice of bytes which represent the private key as PEM encode
func (k *KeyPair) GetPrivatePEM() []byte {
	der := k.GetPrivateDER()
	t := ""
	if k.Type == KeyTypeRSA {
		t = "RSA PRIVATE KEY"
	} else if k.Type == KeyTypeEc {
		t = "EC PRIVATE KEY"
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  t,
		Bytes: der,
	})
}

// Marshal marshal the actual KeyPair pointer to a slice of bytes
func (k *KeyPair) Marshal() []byte {
	cp := new(KeyPair)
	*cp = *k

	cp.Private = k.GetPrivateDER()
	cp.Public = nil

	ret, _ := json.Marshal(cp)

	return ret
}

// UnmarshalKeyPair rebuilds an existing KeyPair pointer marshaled with *KeyPair.Marshal function
func UnmarshalKeyPair(input []byte) (*KeyPair, error) {
	tmp := new(keyPairExport)
	err := json.Unmarshal(input, tmp)
	if err != nil {
		return nil, err
	}

	ret := &KeyPair{
		Type:   tmp.Type,
		Length: tmp.Length,
	}

	switch tmp.Type {
	case KeyTypeRSA:
		privateKey, err := x509.ParsePKCS1PrivateKey(tmp.Private)
		if err != nil {
			return nil, err
		}

		ret.Private = privateKey
		ret.Public = privateKey.Public()
	case KeyTypeEc:
		privateKey, err := x509.ParseECPrivateKey(tmp.Private)
		if err != nil {
			return nil, err
		}

		ret.Private = privateKey
		ret.Public = privateKey.Public()
	}

	return ret, nil
}
