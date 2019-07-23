package replication

import (
	"regexp"

	"github.com/alexandre/securelink"
)

// Those variables defines the default certificates key algorithm and key size
var (
	DefaultCertKeyAlgorithm = securelink.KeyTypeEc
	DefaultCertKeyLength    = securelink.KeyLengthEc384
)

// Are used to check if the client is looking for the raft
// service inside the TLS service
var (
	MemberlistHostNamePrefix      = "memberlist"
	CheckMemberlistHostRequestReg = regexp.MustCompile("^" + MemberlistHostNamePrefix + "\\.")
)
