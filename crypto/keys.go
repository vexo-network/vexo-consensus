package crypto

import (
	"errors"
	"math/big"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

// ValidateConsensusPublicKey checks that publicKey is an admissible consensus key for backend.
// It is pure and deterministic so it can run inside state transitions.
//
//   - deterministic: the key must be non-empty (ErrEmptyPublicKey).
//   - ed25519: the key must be a canonical 32-byte point of the curve that is not of small order
//     (ErrInvalidEd25519PublicKey).
//   - bls: the key must be a valid non-identity G1 point in the prime-order subgroup
//     (ErrInvalidBLSPublicKey) and metadata[BLSProofOfPossessionMetadataKey] must hold a valid
//     proof of possession for it (ErrMissingBLSProof, ErrInvalidBLSProof). Builds without cgo have no
//     BLS backend and report ErrBLSBackendUnavailable.
func ValidateConsensusPublicKey(backend config.CryptoBackend, publicKey types.PublicKey, metadata map[string]string) error {
	switch backend {
	case config.CryptoBackendDeterministic:
		if len(publicKey) == 0 {
			return ErrEmptyPublicKey
		}
		return nil
	case config.CryptoBackendEd25519:
		if !validEd25519PublicKey(publicKey) {
			return ErrInvalidEd25519PublicKey
		}
		return nil
	case config.CryptoBackendBLS:
		proof, err := BLSProofOfPossessionFromMetadata(metadata)
		if err != nil {
			if keyErr := NewBLSTBLSVerifierAdapter().ValidatePublicKey(publicKey); keyErr != nil {
				return blsPublicKeyError(keyErr)
			}
			return err
		}
		return validateBLSCredentialKey(NewBLSTBLSVerifierAdapter(), publicKey, proof)
	default:
		return ErrUnsupportedCryptoBackend
	}
}

// validateBLSCredentialKey is the single definition of an admissible BLS validator key: a valid
// public key whose proof of possession verifies. Consensus-key admission and the verified-key
// registry both use it so they can never disagree.
func validateBLSCredentialKey(adapter BLSAdapter, publicKey types.PublicKey, proof types.Signature) error {
	if err := adapter.ValidatePublicKey(publicKey); err != nil {
		return blsPublicKeyError(err)
	}
	if len(proof) == 0 {
		return ErrMissingBLSProof
	}
	if !adapter.VerifyProofOfPossession(publicKey, proof) {
		return ErrInvalidBLSProof
	}
	return nil
}

func blsPublicKeyError(err error) error {
	if errors.Is(err, ErrBLSBackendUnavailable) {
		return err
	}
	return ErrInvalidBLSPublicKey
}

var (
	ed25519FieldPrime = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	// ed25519CurveD is the twisted Edwards constant d = -121665/121666 of edwards25519.
	ed25519CurveD = new(big.Int).Mod(
		new(big.Int).Mul(big.NewInt(-121665), ed25519FieldInverse(big.NewInt(121666))),
		ed25519FieldPrime,
	)
)

// ed25519FieldInverse returns value^(p-2) mod p, the inverse for non-zero values and zero for zero,
// so the curve arithmetic below has no failure path.
func ed25519FieldInverse(value *big.Int) *big.Int {
	exponent := new(big.Int).Sub(ed25519FieldPrime, big.NewInt(2))
	return new(big.Int).Exp(value, exponent, ed25519FieldPrime)
}

type ed25519Point struct {
	x, y *big.Int
}

// validEd25519PublicKey reports whether encoded is the canonical RFC 8032 encoding of a point on
// edwards25519 whose order does not divide 8. Variable-time arithmetic is acceptable because
// public keys are public data.
func validEd25519PublicKey(encoded []byte) bool {
	point, ok := decodeEd25519Point(encoded)
	if !ok {
		return false
	}
	return !point.hasSmallOrder()
}

func decodeEd25519Point(encoded []byte) (ed25519Point, bool) {
	const size = 32
	if len(encoded) != size {
		return ed25519Point{}, false
	}
	bigEndian := make([]byte, size)
	for index, value := range encoded {
		bigEndian[size-1-index] = value
	}
	signBit := bigEndian[0] >> 7
	bigEndian[0] &= 0x7f
	y := new(big.Int).SetBytes(bigEndian)
	if y.Cmp(ed25519FieldPrime) >= 0 {
		return ed25519Point{}, false
	}
	// x^2 = (y^2 - 1) / (d*y^2 + 1); the denominator is never zero because d is a non-square.
	ySquared := new(big.Int).Mul(y, y)
	ySquared.Mod(ySquared, ed25519FieldPrime)
	numerator := new(big.Int).Sub(ySquared, big.NewInt(1))
	numerator.Mod(numerator, ed25519FieldPrime)
	denominator := new(big.Int).Mul(ed25519CurveD, ySquared)
	denominator.Add(denominator, big.NewInt(1))
	denominator.Mod(denominator, ed25519FieldPrime)
	xSquared := numerator.Mul(numerator, ed25519FieldInverse(denominator))
	xSquared.Mod(xSquared, ed25519FieldPrime)
	x := new(big.Int).ModSqrt(xSquared, ed25519FieldPrime)
	if x == nil {
		return ed25519Point{}, false
	}
	if x.Sign() == 0 && signBit == 1 {
		return ed25519Point{}, false
	}
	if uint(x.Bit(0)) != uint(signBit) {
		x.Sub(ed25519FieldPrime, x)
	}
	return ed25519Point{x: x, y: y}, true
}

// hasSmallOrder reports whether [8]P is the identity, that is whether the order of P divides 8.
func (point ed25519Point) hasSmallOrder() bool {
	current := point
	for range 3 {
		current = current.double()
	}
	return current.x.Sign() == 0 && current.y.Cmp(big.NewInt(1)) == 0
}

// double applies the complete twisted Edwards doubling law for a = -1:
// x3 = 2xy / (1 + d x^2 y^2), y3 = (y^2 + x^2) / (1 - d x^2 y^2).
func (point ed25519Point) double() ed25519Point {
	prime := ed25519FieldPrime
	xx := new(big.Int).Mul(point.x, point.x)
	xx.Mod(xx, prime)
	yy := new(big.Int).Mul(point.y, point.y)
	yy.Mod(yy, prime)
	dxxyy := new(big.Int).Mul(xx, yy)
	dxxyy.Mul(dxxyy, ed25519CurveD)
	dxxyy.Mod(dxxyy, prime)

	xNumerator := new(big.Int).Mul(point.x, point.y)
	xNumerator.Lsh(xNumerator, 1)
	xNumerator.Mod(xNumerator, prime)
	xDenominator := new(big.Int).Add(big.NewInt(1), dxxyy)
	xDenominator.Mod(xDenominator, prime)

	yNumerator := new(big.Int).Add(yy, xx)
	yNumerator.Mod(yNumerator, prime)
	yDenominator := new(big.Int).Sub(big.NewInt(1), dxxyy)
	yDenominator.Mod(yDenominator, prime)

	x := xNumerator.Mul(xNumerator, ed25519FieldInverse(xDenominator))
	x.Mod(x, prime)
	y := yNumerator.Mul(yNumerator, ed25519FieldInverse(yDenominator))
	y.Mod(y, prime)
	return ed25519Point{x: x, y: y}
}
