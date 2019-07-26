// +build go1.1 go1.2 go1.3 go1.4 go1.5 go1.6 go1.7 go1.8 go1.9 go1.10 go1.11 go1.12

package securelink_test

import (
	"reflect"
	"testing"

	"github.com/alexandrestein/securelink"
)

func TestMarshalKeyPairs(t *testing.T) {
	tests := []struct {
		Name   string
		Type   securelink.KeyType
		Length securelink.KeyLength
		Long   bool
		Error  bool
	}{
		{"EC 256", securelink.KeyTypeEc, securelink.KeyLengthEc256, false, false},
		{"EC 384", securelink.KeyTypeEc, securelink.KeyLengthEc384, false, false},
		{"EC 521", securelink.KeyTypeEc, securelink.KeyLengthEc521, false, false},

		{"RSA 2048", securelink.KeyTypeRSA, securelink.KeyLengthRsa2048, false, false},
		{"RSA 3072", securelink.KeyTypeRSA, securelink.KeyLengthRsa3072, true, false},
		{"RSA 4096", securelink.KeyTypeRSA, securelink.KeyLengthRsa4096, true, false},
		{"RSA 8192", securelink.KeyTypeRSA, securelink.KeyLengthRsa8192, true, false},

		{"not valid", securelink.KeyType(""), securelink.KeyLengthRsa8192, false, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if test.Long && testing.Short() {
				t.SkipNow()
			}

			keyPair, err := securelink.NewKeyPair(test.Type, test.Length)
			if err != nil {
				if test.Error {
					return
				}
				t.Fatal(err)
			}
			buf := keyPair.Marshal()

			loaded, err := securelink.UnmarshalKeyPair(buf)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(loaded, keyPair) {
				t.Fatalf("the key pairs must be equal but are not: %v %v", loaded, keyPair)
			}
		})
	}
}