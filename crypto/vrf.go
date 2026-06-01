package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidVRFKey = errors.New("invalid vrf key")

type DeterministicVRF struct {
	keys map[string][]byte
}

func NewDeterministicVRF(keys map[string][]byte) DeterministicVRF {
	copied := make(map[string][]byte, len(keys))
	for publicKey, privateKey := range keys {
		copied[publicKey] = append([]byte(nil), privateKey...)
	}
	return DeterministicVRF{keys: copied}
}

func (vrf DeterministicVRF) Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found || len(privateKey) == 0 {
		return nil, nil, ErrInvalidVRFKey
	}
	output = vrfOutput(privateKey, seed)
	proof = append([]byte(nil), output...)
	return output, proof, nil
}

func (vrf DeterministicVRF) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found || len(privateKey) == 0 {
		return false
	}
	expected := vrfOutput(privateKey, seed)
	return hmac.Equal(expected, output) && hmac.Equal(expected, proof)
}

func vrfOutput(privateKey []byte, seed []byte) []byte {
	mac := hmac.New(sha256.New, privateKey)
	mac.Write(seed)
	return mac.Sum(nil)
}
