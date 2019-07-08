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

	"crypto/ed25519"
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
	case KeyTypeEd25519:
		return NewEd25519(), nil
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

func NewEd25519() *KeyPair {
	ret := newKeyPair(KeyTypeEd25519, KeyLengthEd25519)

	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	ret.Private = privateKey
	ret.Public = publicKey

	return ret
}

// GetPrivateDER returns a slice of bytes which represent the private key as DER encoded
func (k *KeyPair) GetPrivateDER() []byte {
	ret, _ := x509.MarshalPKCS8PrivateKey(k.Private)
	return ret
}

// GetPrivatePEM returns a slice of bytes which represent the private key as PEM encode
func (k *KeyPair) GetPrivatePEM() []byte {
	der := k.GetPrivateDER()
	t := ""
	switch k.Type {
	case KeyTypeEc, KeyTypeEd25519:
		t = "EC PRIVATE KEY"
	case KeyTypeRSA:
		t = "RSA PRIVATE KEY"
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  t,
		Bytes: der,
	})
}

// Marshal marshal the actual KeyPair pointer to a slice of bytes
func (k *KeyPair) Marshal() []byte {
	cp := new(keyPairExport)
	cp.KeyPair = k

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

	privateKey, err := x509.ParsePKCS8PrivateKey(tmp.Private)
	if err != nil {
		return nil, err
	}

	switch tmp.Type {
	case KeyTypeEd25519:
		privateEd25519Key, ok := privateKey.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not a valid curve 25519 key")
		}

		ret.Private = privateEd25519Key
		ret.Public = privateEd25519Key.Public()
	case KeyTypeRSA:
		privateRSAKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not a valid RSA key")
		}

		ret.Private = privateRSAKey
		ret.Public = privateRSAKey.Public()
	case KeyTypeEc:
		privateEcKey, ok := privateKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not a valid EC key")
		}

		ret.Private = privateEcKey
		ret.Public = privateEcKey.Public()
	}

	return ret, nil
}
