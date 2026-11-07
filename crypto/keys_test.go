package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

// smallOrderEd25519Encodings are the canonical encodings of the eight points of order dividing 8.
var smallOrderEd25519Encodings = []string{
	"0100000000000000000000000000000000000000000000000000000000000000",
	"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
	"0000000000000000000000000000000000000000000000000000000000000000",
	"0000000000000000000000000000000000000000000000000000000000000080",
	"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
	"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa",
	"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
	"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85",
}

func ed25519YEncoding(y *big.Int) []byte {
	encoded := make([]byte, 32)
	bigEndian := y.FillBytes(make([]byte, 32))
	for index, value := range bigEndian {
		encoded[31-index] = value
	}
	return encoded
}

func TestValidateConsensusPublicKeyEd25519AcceptsGeneratedKeys(t *testing.T) {
	for range 16 {
		publicKey, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, types.PublicKey(publicKey), nil); err != nil {
			t.Fatalf("expected generated ed25519 key to be valid: %v", err)
		}
	}
	for _, vector := range []string{
		"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a", // RFC 8032 test 1
		"3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c", // RFC 8032 test 2
	} {
		publicKey, err := hex.DecodeString(vector)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, publicKey, nil); err != nil {
			t.Fatalf("expected RFC 8032 key %s to be valid: %v", vector, err)
		}
	}
}

func TestValidateConsensusPublicKeyEd25519RejectsMalformedKeys(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]types.PublicKey{
		"nil":     nil,
		"empty":   {},
		"short":   types.PublicKey(publicKey[:ed25519.PublicKeySize-1]),
		"long":    append(types.PublicKey(publicKey), 0),
		"31 byte": make(types.PublicKey, 31),
	} {
		if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, candidate, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
			t.Fatalf("%s: expected invalid ed25519 public key, got %v", name, err)
		}
	}
}

func TestValidateConsensusPublicKeyEd25519RejectsSmallOrderPoints(t *testing.T) {
	for _, vector := range smallOrderEd25519Encodings {
		encoded, err := hex.DecodeString(vector)
		if err != nil {
			t.Fatal(err)
		}
		point, ok := decodeEd25519Point(encoded)
		if !ok {
			t.Fatalf("expected %s to be an on-curve point", vector)
		}
		if !point.hasSmallOrder() {
			t.Fatalf("expected %s to have small order", vector)
		}
		if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, encoded, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
			t.Fatalf("expected small-order key %s to be rejected, got %v", vector, err)
		}
	}
}

func TestValidateConsensusPublicKeyEd25519RejectsNonCanonicalAndOffCurveEncodings(t *testing.T) {
	var canonical, noncanonical, offCurve []byte
	for y := int64(2); y < 19 && (canonical == nil || offCurve == nil); y++ {
		encoded := ed25519YEncoding(big.NewInt(y))
		if _, ok := decodeEd25519Point(encoded); ok {
			if canonical == nil {
				canonical = encoded
				noncanonical = ed25519YEncoding(new(big.Int).Add(ed25519FieldPrime, big.NewInt(y)))
			}
		} else if offCurve == nil {
			offCurve = encoded
		}
	}
	if canonical == nil || offCurve == nil {
		t.Fatal("expected small y values with and without a curve point")
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, canonical, nil); err != nil {
		t.Fatalf("expected canonical point to be valid: %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, noncanonical, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
		t.Fatalf("expected non-canonical y to be rejected, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, offCurve, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
		t.Fatalf("expected off-curve y to be rejected, got %v", err)
	}
	negativeZero := ed25519YEncoding(big.NewInt(1))
	negativeZero[31] |= 0x80
	if err := ValidateConsensusPublicKey(config.CryptoBackendEd25519, negativeZero, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
		t.Fatalf("expected x=0 with the sign bit set to be rejected, got %v", err)
	}
}

func TestValidateConsensusPublicKeyDeterministicRequiresKey(t *testing.T) {
	if err := ValidateConsensusPublicKey(config.CryptoBackendDeterministic, nil, nil); !errors.Is(err, ErrEmptyPublicKey) {
		t.Fatalf("expected empty public key, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendDeterministic, types.PublicKey("alice"), nil); err != nil {
		t.Fatal(err)
	}
}

func TestValidateConsensusPublicKeyRejectsUnknownBackend(t *testing.T) {
	if err := ValidateConsensusPublicKey("unknown", types.PublicKey("alice"), nil); !errors.Is(err, ErrUnsupportedCryptoBackend) {
		t.Fatalf("expected unsupported backend, got %v", err)
	}
}
