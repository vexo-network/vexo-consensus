package address

import (
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

const (
	AccountHRP            = "vexo"
	ValidatorOperatorHRP  = "vexovaloper"
	ValidatorConsensusHRP = "vexovalcons"
	addressLength         = 20
	addressDomain         = "vexo.address.v1"
)

var (
	ErrEmptyPublicKey  = errors.New("public key is empty")
	ErrInvalidAddress  = errors.New("invalid address")
	ErrInvalidPrefix   = errors.New("invalid address prefix")
	ErrAddressMismatch = errors.New("address does not match public key")
)

func AccountFromPublicKey(publicKey types.PublicKey) (types.Address, error) {
	return FromPublicKey(AccountHRP, publicKey)
}

func ValidatorOperatorFromPublicKey(publicKey types.PublicKey) (types.Address, error) {
	return FromPublicKey(ValidatorOperatorHRP, publicKey)
}

func ValidatorConsensusFromPublicKey(publicKey types.PublicKey) (types.Address, error) {
	return FromPublicKey(ValidatorConsensusHRP, publicKey)
}

func FromPublicKey(hrp string, publicKey types.PublicKey) (types.Address, error) {
	if len(publicKey) == 0 {
		return "", ErrEmptyPublicKey
	}
	if err := validateHRP(hrp); err != nil {
		return "", err
	}
	digest := sha256.Sum256(append(append([]byte(addressDomain+":"+hrp+":"), publicKey...), '\n'))
	encoded, err := bech32Encode(hrp, digest[:addressLength])
	if err != nil {
		return "", err
	}
	return types.Address(encoded), nil
}

func Validate(value types.Address, expectedHRP string) error {
	hrp, data, err := bech32Decode(string(value))
	if err != nil {
		return err
	}
	if expectedHRP != "" && hrp != expectedHRP {
		return ErrInvalidPrefix
	}
	if len(data) != addressLength {
		return ErrInvalidAddress
	}
	return nil
}

func MatchesPublicKey(value types.Address, hrp string, publicKey types.PublicKey) error {
	expected, err := FromPublicKey(hrp, publicKey)
	if err != nil {
		return err
	}
	if string(value) != string(expected) {
		return ErrAddressMismatch
	}
	return nil
}

func HRP(value types.Address) (string, error) {
	hrp, _, err := bech32Decode(string(value))
	return hrp, err
}

func validateHRP(hrp string) error {
	if hrp == "" || strings.ToLower(hrp) != hrp {
		return ErrInvalidPrefix
	}
	for _, char := range hrp {
		if char < 33 || char > 126 {
			return ErrInvalidPrefix
		}
	}
	return nil
}
