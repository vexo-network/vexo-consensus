package p2p

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/vexo-network/vexo-consensus/types"
)

const (
	nodeIDDomain = "vexo.p2p.node-id.v1\x00"
	nodeIDBytes  = 20

	// MaxPeerIDBytes bounds the length of a peer identity that is accepted
	// from the network or persisted in peer tables.
	MaxPeerIDBytes = 128
)

// NodeIDFromPublicKey derives the self-certifying peer identity of a node
// public key. A peer that presents this identity and proves possession of the
// matching private key cannot be impersonated without breaking SHA-256.
func NodeIDFromPublicKey(publicKey types.PublicKey) PeerID {
	sum := sha256.Sum256(append([]byte(nodeIDDomain), publicKey...))
	return PeerID(hex.EncodeToString(sum[:nodeIDBytes]))
}

// ValidPeerID reports whether id is a bounded printable identity that is safe
// to store, log, and embed in signed transcripts.
func ValidPeerID(id PeerID) bool {
	if id == "" || len(id) > MaxPeerIDBytes {
		return false
	}
	for index := 0; index < len(id); index++ {
		if id[index] <= ' ' || id[index] >= 0x7f {
			return false
		}
	}
	return true
}

// IsDerivedNodeID reports whether id has the form NodeIDFromPublicKey
// produces, so a peer id can be recognised as key-bound without its key.
func IsDerivedNodeID(id PeerID) bool {
	if len(id) != 2*nodeIDBytes {
		return false
	}
	for index := 0; index < len(id); index++ {
		character := id[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
