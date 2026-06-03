package crypto

import (
	"encoding/base64"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const BLSProofOfPossessionMetadataKey = "bls_pop"

var (
	ErrMissingBLSValidatorID    = errors.New("bls validator id is required")
	ErrMissingBLSPublicKey      = errors.New("bls public key is required")
	ErrMissingBLSProof          = errors.New("bls proof of possession is required")
	ErrInvalidBLSPublicKey      = errors.New("bls public key is invalid")
	ErrInvalidBLSProof          = errors.New("bls proof of possession is invalid")
	ErrDuplicateBLSPublicKey    = errors.New("duplicate bls public key")
	ErrUnregisteredBLSPublicKey = errors.New("unregistered bls public key")
)

type BLSValidatorCredential struct {
	ValidatorID       types.ValidatorID
	PublicKey         types.PublicKey
	ProofOfPossession types.Signature
}

type BLSAggregateVerifier struct {
	adapter        BLSAdapter
	registeredKeys map[string]struct{}
}

func NewBLSAggregateVerifier(adapter BLSAdapter, credentials []BLSValidatorCredential) (BLSAggregateVerifier, error) {
	if err := ValidateBLSAdapter(adapter); err != nil {
		return BLSAggregateVerifier{}, err
	}
	registeredKeys, err := ValidateBLSValidatorCredentials(adapter, credentials)
	if err != nil {
		return BLSAggregateVerifier{}, err
	}
	return BLSAggregateVerifier{adapter: adapter, registeredKeys: registeredKeys}, nil
}

func (verifier BLSAggregateVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if verifier.adapter == nil || len(publicKeys) == 0 {
		return false
	}
	for _, publicKey := range publicKeys {
		if len(publicKey) == 0 {
			return false
		}
		if _, found := verifier.registeredKeys[string(publicKey)]; !found {
			return false
		}
		if err := verifier.adapter.ValidatePublicKey(publicKey); err != nil {
			return false
		}
	}
	return verifier.adapter.VerifyAggregate(publicKeys, message, signature)
}

func ValidateBLSValidatorSet(adapter BLSAdapter, validatorSet validator.Set) ([]BLSValidatorCredential, error) {
	if validatorSet == nil {
		return nil, ErrMissingBLSPublicKey
	}
	validators := validatorSet.List()
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
	if _, err := ValidateBLSValidatorCredentials(adapter, credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

func ValidateBLSValidatorCredentials(adapter BLSAdapter, credentials []BLSValidatorCredential) (map[string]struct{}, error) {
	if err := ValidateBLSAdapter(adapter); err != nil {
		return nil, err
	}
	registeredKeys := make(map[string]struct{}, len(credentials))
	for _, credential := range credentials {
		if credential.ValidatorID == "" {
			return nil, ErrMissingBLSValidatorID
		}
		if len(credential.PublicKey) == 0 {
			return nil, ErrMissingBLSPublicKey
		}
		if len(credential.ProofOfPossession) == 0 {
			return nil, ErrMissingBLSProof
		}
		key := string(credential.PublicKey)
		if _, found := registeredKeys[key]; found {
			return nil, ErrDuplicateBLSPublicKey
		}
		if err := adapter.ValidatePublicKey(credential.PublicKey); err != nil {
			return nil, ErrInvalidBLSPublicKey
		}
		if !adapter.VerifyProofOfPossession(credential.PublicKey, credential.ProofOfPossession) {
			return nil, ErrInvalidBLSProof
		}
		registeredKeys[key] = struct{}{}
	}
	return registeredKeys, nil
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
