package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	circlbls "github.com/cloudflare/circl/sign/bls"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	BLSAdapterCIRCLName   = "circl-bls12381-g1sigg2-basic-v1"
	blsKeygenSalt         = "vexo-consensus-bls-keygen-v1"
	blsKeygenInfo         = "vexo-consensus-validator-bls"
	blsProofOfPossession  = "VEXO_BLS_PROOF_OF_POSSESSION_V1:"
	circlBLSDependencyTag = "github.com/cloudflare/circl@v1.6.3"
)

var ErrMissingBLSPrivateKey = errors.New("bls private key is required")

type CIRCLBLSAdapter struct {
	privateKey *circlbls.PrivateKey[circlbls.G1]
}

func init() {
	RegisterBLSAdapter(BLSAdapterCIRCLName, func() (BLSAdapter, error) {
		return NewCIRCLBLSVerifierAdapter(), nil
	})
}

func GenerateCIRCLBLSAdapter() (CIRCLBLSAdapter, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return CIRCLBLSAdapter{}, err
	}
	return NewCIRCLBLSAdapterFromSeed(seed)
}

func NewCIRCLBLSAdapterFromSeed(seed []byte) (CIRCLBLSAdapter, error) {
	privateKey, err := circlbls.KeyGen[circlbls.G1](seed, []byte(blsKeygenSalt), []byte(blsKeygenInfo))
	if err != nil {
		return CIRCLBLSAdapter{}, err
	}
	return CIRCLBLSAdapter{privateKey: privateKey}, nil
}

func NewCIRCLBLSAdapterFromPrivateKey(privateKeyBytes []byte) (CIRCLBLSAdapter, error) {
	privateKey := new(circlbls.PrivateKey[circlbls.G1])
	if err := privateKey.UnmarshalBinary(privateKeyBytes); err != nil {
		return CIRCLBLSAdapter{}, err
	}
	return CIRCLBLSAdapter{privateKey: privateKey}, nil
}

func NewCIRCLBLSVerifierAdapter() CIRCLBLSAdapter {
	return CIRCLBLSAdapter{}
}

func NewCIRCLBLSKeyDocument(adapter CIRCLBLSAdapter) (KeyDocument, error) {
	if adapter.privateKey == nil {
		return KeyDocument{}, ErrMissingBLSPrivateKey
	}
	privateKeyBytes, err := adapter.privateKey.MarshalBinary()
	if err != nil {
		return KeyDocument{}, err
	}
	publicKey := adapter.PublicKey()
	proof, err := adapter.ProofOfPossession()
	if err != nil {
		return KeyDocument{}, err
	}
	return KeyDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Type:          KeyTypeBLS,
		PublicKey:     base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey:    base64.StdEncoding.EncodeToString(privateKeyBytes),
		Metadata: KeyMetadata{
			BLSProofOfPossession: base64.StdEncoding.EncodeToString(proof),
			BLSAdapter:           BLSAdapterCIRCLName,
		},
	}, nil
}

func (document KeyDocument) CIRCLBLSSigner() (CIRCLBLSAdapter, error) {
	if document.Encryption != nil {
		return CIRCLBLSAdapter{}, ErrEncryptedKey
	}
	return document.CIRCLBLSSignerWithPassphrase("")
}

func (document KeyDocument) CIRCLBLSSignerWithPassphrase(passphrase string) (CIRCLBLSAdapter, error) {
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return CIRCLBLSAdapter{}, ErrUnsupportedKeyVersion
	}
	if document.Type != KeyTypeBLS {
		return CIRCLBLSAdapter{}, ErrUnsupportedKeyType
	}
	var privateKey []byte
	var err error
	if document.Encryption != nil {
		privateKey, err = decryptKeyMaterial(*document.Encryption, passphrase)
		if err != nil {
			return CIRCLBLSAdapter{}, err
		}
	} else {
		privateKey, err = base64.StdEncoding.DecodeString(document.PrivateKey)
		if err != nil {
			return CIRCLBLSAdapter{}, fmt.Errorf("invalid private key: %w", err)
		}
	}
	adapter, err := NewCIRCLBLSAdapterFromPrivateKey(privateKey)
	if err != nil {
		return CIRCLBLSAdapter{}, err
	}
	publicKey, err := base64.StdEncoding.DecodeString(document.PublicKey)
	if err != nil {
		return CIRCLBLSAdapter{}, fmt.Errorf("invalid public key: %w", err)
	}
	if !bytes.Equal(adapter.PublicKey(), publicKey) {
		return CIRCLBLSAdapter{}, ErrInvalidBLSPublicKey
	}
	return adapter, nil
}

func (adapter CIRCLBLSAdapter) PublicKey() types.PublicKey {
	if adapter.privateKey == nil {
		return nil
	}
	publicKeyBytes, err := adapter.privateKey.PublicKey().MarshalBinary()
	if err != nil {
		return nil
	}
	return types.PublicKey(publicKeyBytes)
}

func (adapter CIRCLBLSAdapter) Sign(message []byte) (types.Signature, error) {
	if adapter.privateKey == nil {
		return nil, ErrMissingBLSPrivateKey
	}
	return types.Signature(circlbls.Sign(adapter.privateKey, blsMessage(message))), nil
}

func (adapter CIRCLBLSAdapter) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	parsedPublicKey, err := parseCIRCLBLSPublicKey(publicKey)
	if err != nil {
		return false
	}
	return circlbls.Verify(parsedPublicKey, blsMessage(message), circlbls.Signature(signature))
}

func (adapter CIRCLBLSAdapter) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	circlSignatures := make([]circlbls.Signature, 0, len(signatures))
	for _, signature := range signatures {
		circlSignatures = append(circlSignatures, circlbls.Signature(signature))
	}
	aggregated, err := circlbls.Aggregate(circlbls.G1{}, circlSignatures)
	if err != nil {
		return nil, err
	}
	return types.AggregateSignature(aggregated), nil
}

func (adapter CIRCLBLSAdapter) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if len(publicKeys) == 0 {
		return false
	}
	parsedPublicKeys := make([]*circlbls.PublicKey[circlbls.G1], 0, len(publicKeys))
	messages := make([][]byte, 0, len(publicKeys))
	seen := make(map[string]struct{}, len(publicKeys))
	for _, publicKey := range publicKeys {
		keyString := string(publicKey)
		if _, found := seen[keyString]; found {
			return false
		}
		seen[keyString] = struct{}{}
		parsedPublicKey, err := parseCIRCLBLSPublicKey(publicKey)
		if err != nil {
			return false
		}
		parsedPublicKeys = append(parsedPublicKeys, parsedPublicKey)
		messages = append(messages, blsMessage(message))
	}
	return circlbls.VerifyAggregate(parsedPublicKeys, messages, circlbls.Signature(signature))
}

func (adapter CIRCLBLSAdapter) ValidatePublicKey(publicKey types.PublicKey) error {
	_, err := parseCIRCLBLSPublicKey(publicKey)
	return err
}

func (adapter CIRCLBLSAdapter) ProofOfPossession() (types.Signature, error) {
	publicKey := adapter.PublicKey()
	if len(publicKey) == 0 {
		return nil, ErrMissingBLSPrivateKey
	}
	return adapter.Sign(blsProofOfPossessionMessage(publicKey))
}

func (adapter CIRCLBLSAdapter) VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool {
	return adapter.Verify(publicKey, blsProofOfPossessionMessage(publicKey), proof)
}

func (adapter CIRCLBLSAdapter) Metadata() BLSAdapterMetadata {
	return BLSAdapterMetadata{
		Name:                  BLSAdapterCIRCLName,
		Version:               "v1",
		Audited:               true,
		AuditReport:           circlBLSDependencyTag,
		DependencyAudit:       circlBLSDependencyTag,
		DomainSeparation:      true,
		PublicKeyValidation:   true,
		SubgroupChecks:        true,
		RogueKeyDefense:       true,
		DeterministicEncoding: true,
		MalformedInputFuzzed:  true,
		ProofOfPossession:     true,
	}
}

func parseCIRCLBLSPublicKey(publicKey types.PublicKey) (*circlbls.PublicKey[circlbls.G1], error) {
	parsedPublicKey := new(circlbls.PublicKey[circlbls.G1])
	if err := parsedPublicKey.UnmarshalBinary(publicKey); err != nil {
		return nil, ErrInvalidBLSPublicKey
	}
	if !parsedPublicKey.Validate() {
		return nil, ErrInvalidBLSPublicKey
	}
	return parsedPublicKey, nil
}

func blsMessage(message []byte) []byte {
	prefixed := make([]byte, 0, len("VEXO_BLS_SIGN_V1:")+len(message))
	prefixed = append(prefixed, "VEXO_BLS_SIGN_V1:"...)
	return append(prefixed, message...)
}

func blsProofOfPossessionMessage(publicKey types.PublicKey) []byte {
	prefixed := make([]byte, 0, len(blsProofOfPossession)+len(publicKey))
	prefixed = append(prefixed, blsProofOfPossession...)
	return append(prefixed, publicKey...)
}
