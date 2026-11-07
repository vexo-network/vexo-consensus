package crypto

import (
	"encoding/base64"
	"errors"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const BLSProofOfPossessionMetadataKey = "bls_pop"

var (
	ErrMissingBLSValidatorID = errors.New("bls validator id is required")
	ErrMissingBLSPublicKey   = errors.New("bls public key is required")
	ErrMissingBLSProof       = errors.New("bls proof of possession is required")
	ErrInvalidBLSPublicKey   = errors.New("bls public key is invalid")
	ErrInvalidBLSProof       = errors.New("bls proof of possession is invalid")
	ErrDuplicateBLSPublicKey = errors.New("duplicate bls public key")
)

type BLSValidatorCredential struct {
	ValidatorID       types.ValidatorID
	PublicKey         types.PublicKey
	ProofOfPossession types.Signature
}

// BLSKeyRegistry holds the BLS public keys whose proof of possession has been verified and
// verifies aggregate signatures only over registered keys. Same-message aggregation is only sound
// against rogue-key attacks when every aggregated key has proven possession of its secret, so an
// unregistered key never verifies: the registry fails closed.
//
// Validator keys change with the validator set, so keys are registered per validator-set snapshot
// with Register or RegisterValidators before that snapshot is used to verify a certificate.
// Verified keys are kept for the life of the registry because certificates for older heights stay
// verifiable against the older validator sets. The registry is safe for concurrent use.
type BLSKeyRegistry struct {
	adapter  BLSAdapter
	mu       sync.RWMutex
	verified map[string]string
}

func NewBLSKeyRegistry(adapter BLSAdapter) (*BLSKeyRegistry, error) {
	if err := ValidateBLSAdapter(adapter); err != nil {
		return nil, err
	}
	return &BLSKeyRegistry{adapter: adapter, verified: make(map[string]string)}, nil
}

// Register verifies the credentials of one validator-set snapshot and records their keys. It is
// all-or-nothing: when any credential is invalid, no key of the call is recorded. Credentials that
// were already verified with the same proof are not verified again.
func (registry *BLSKeyRegistry) Register(credentials []BLSValidatorCredential) error {
	snapshotKeys := make(map[string]struct{}, len(credentials))
	newlyVerified := make(map[string]string, len(credentials))
	for _, credential := range credentials {
		if credential.ValidatorID == "" {
			return ErrMissingBLSValidatorID
		}
		if len(credential.PublicKey) == 0 {
			return ErrMissingBLSPublicKey
		}
		if len(credential.ProofOfPossession) == 0 {
			return ErrMissingBLSProof
		}
		key := string(credential.PublicKey)
		if _, duplicate := snapshotKeys[key]; duplicate {
			return ErrDuplicateBLSPublicKey
		}
		snapshotKeys[key] = struct{}{}
		if registry.verifiedWith(key, string(credential.ProofOfPossession)) {
			continue
		}
		if err := validateBLSCredentialKey(registry.adapter, credential.PublicKey, credential.ProofOfPossession); err != nil {
			return err
		}
		newlyVerified[key] = string(credential.ProofOfPossession)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for key, proof := range newlyVerified {
		registry.verified[key] = proof
	}
	return nil
}

// RegisterValidators registers the keys of one validator-set snapshot from the validators'
// bls_pop metadata.
func (registry *BLSKeyRegistry) RegisterValidators(validators []validator.Validator) error {
	credentials, err := BLSValidatorCredentialsFromValidators(validators)
	if err != nil {
		return err
	}
	return registry.Register(credentials)
}

// VerifyAggregate verifies an aggregate signature over message by publicKeys. It returns false
// unless every key has been registered with a verified proof of possession.
func (registry *BLSKeyRegistry) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if len(publicKeys) == 0 || !registry.allRegistered(publicKeys) {
		return false
	}
	return registry.adapter.VerifyAggregate(publicKeys, message, signature)
}

func (registry *BLSKeyRegistry) verifiedWith(key string, proof string) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	verifiedProof, found := registry.verified[key]
	return found && verifiedProof == proof
}

func (registry *BLSKeyRegistry) allRegistered(publicKeys []types.PublicKey) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	for _, publicKey := range publicKeys {
		if _, found := registry.verified[string(publicKey)]; !found {
			return false
		}
	}
	return true
}

func BLSValidatorCredentialsFromValidators(validators []validator.Validator) ([]BLSValidatorCredential, error) {
	credentials := make([]BLSValidatorCredential, 0, len(validators))
	for _, validatorInfo := range validators {
		proof, err := BLSProofOfPossessionFromMetadata(validatorInfo.Metadata)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, BLSValidatorCredential{
			ValidatorID:       validatorInfo.ID,
			PublicKey:         append(types.PublicKey(nil), validatorInfo.PublicKey...),
			ProofOfPossession: proof,
		})
	}
	return credentials, nil
}

func BLSProofOfPossessionFromMetadata(metadata map[string]string) (types.Signature, error) {
	if metadata == nil || metadata[BLSProofOfPossessionMetadataKey] == "" {
		return nil, ErrMissingBLSProof
	}
	proof, err := base64.StdEncoding.DecodeString(metadata[BLSProofOfPossessionMetadataKey])
	if err != nil || len(proof) == 0 {
		return nil, ErrInvalidBLSProof
	}
	return types.Signature(proof), nil
}
