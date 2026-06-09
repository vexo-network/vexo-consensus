package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	blst "github.com/supranational/blst/bindings/go"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	BLSAdapterBLSTName     = "blst-bls12381-minpk-v1"
	blstBLSVersion         = "v1"
	blstBLSDependencyTag   = "github.com/supranational/blst@v0.3.16"
	blstBLSAuditReportTag  = "ncc-group-blst-security-assessment"
	blstBLSMessageDST      = "VEXO_BLS12381G2_XMD:SHA-256_SSWU_RO_V1"
	blstBLSKeyInfo         = "vexo-consensus-validator-blst-minpk-v1"
	blstBLSPrivateKeyBytes = blst.BLST_SCALAR_BYTES
	blstBLSPublicKeyBytes  = blst.BLST_P1_COMPRESS_BYTES
	blstBLSSignatureBytes  = blst.BLST_P2_COMPRESS_BYTES
)

type BLSTBLSAdapter struct {
	privateKey *blst.SecretKey
}

func init() {
	RegisterBLSAdapter(BLSAdapterBLSTName, func() (BLSAdapter, error) {
		return NewBLSTBLSVerifierAdapter(), nil
	})
}

func GenerateBLSTBLSAdapter() (BLSTBLSAdapter, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return BLSTBLSAdapter{}, err
	}
	return NewBLSTBLSAdapterFromSeed(seed)
}

func NewBLSTBLSAdapterFromSeed(seed []byte) (BLSTBLSAdapter, error) {
	privateKey := blst.KeyGen(seed, []byte(blstBLSKeyInfo))
	if privateKey == nil {
		return BLSTBLSAdapter{}, ErrMissingBLSPrivateKey
	}
	return BLSTBLSAdapter{privateKey: privateKey}, nil
}

func NewBLSTBLSAdapterFromPrivateKey(privateKeyBytes []byte) (BLSTBLSAdapter, error) {
	if len(privateKeyBytes) != blstBLSPrivateKeyBytes {
		return BLSTBLSAdapter{}, ErrMissingBLSPrivateKey
	}
	privateKey := new(blst.SecretKey)
	if privateKey.Deserialize(privateKeyBytes) == nil {
		return BLSTBLSAdapter{}, ErrMissingBLSPrivateKey
	}
	return BLSTBLSAdapter{privateKey: privateKey}, nil
}

func NewBLSTBLSVerifierAdapter() BLSTBLSAdapter {
	return BLSTBLSAdapter{}
}

func NewBLSTBLSKeyDocument(adapter BLSTBLSAdapter) (KeyDocument, error) {
	if adapter.privateKey == nil {
		return KeyDocument{}, ErrMissingBLSPrivateKey
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
		PrivateKey:    base64.StdEncoding.EncodeToString(adapter.privateKey.Serialize()),
		Metadata: KeyMetadata{
			BLSProofOfPossession: base64.StdEncoding.EncodeToString(proof),
			BLSAdapter:           BLSAdapterBLSTName,
		},
	}, nil
}

func (document KeyDocument) BLSTBLSSigner() (BLSTBLSAdapter, error) {
	if document.Encryption != nil {
		return BLSTBLSAdapter{}, ErrEncryptedKey
	}
	return document.BLSTBLSSignerWithPassphrase("")
}

func (document KeyDocument) BLSTBLSSignerWithPassphrase(passphrase string) (BLSTBLSAdapter, error) {
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return BLSTBLSAdapter{}, ErrUnsupportedKeyVersion
	}
	if document.Type != KeyTypeBLS {
		return BLSTBLSAdapter{}, ErrUnsupportedKeyType
	}
	var privateKey []byte
	var err error
	if document.Encryption != nil {
		privateKey, err = decryptKeyMaterial(*document.Encryption, passphrase)
		if err != nil {
			return BLSTBLSAdapter{}, err
		}
	} else {
		privateKey, err = base64.StdEncoding.DecodeString(document.PrivateKey)
		if err != nil {
			return BLSTBLSAdapter{}, fmt.Errorf("invalid private key: %w", err)
		}
	}
	adapter, err := NewBLSTBLSAdapterFromPrivateKey(privateKey)
	if err != nil {
		return BLSTBLSAdapter{}, err
	}
	publicKey, err := base64.StdEncoding.DecodeString(document.PublicKey)
	if err != nil {
		return BLSTBLSAdapter{}, fmt.Errorf("invalid public key: %w", err)
	}
	if !bytes.Equal(adapter.PublicKey(), publicKey) {
		return BLSTBLSAdapter{}, ErrInvalidBLSPublicKey
	}
	return adapter, nil
}

func (adapter BLSTBLSAdapter) PublicKey() types.PublicKey {
	if adapter.privateKey == nil {
		return nil
	}
	publicKey := new(blst.P1Affine).From(adapter.privateKey)
	return types.PublicKey(publicKey.Compress())
}

func (adapter BLSTBLSAdapter) Sign(message []byte) (types.Signature, error) {
	if adapter.privateKey == nil {
		return nil, ErrMissingBLSPrivateKey
	}
	signature := new(blst.P2Affine).Sign(adapter.privateKey, blsMessage(message), []byte(blstBLSMessageDST), true)
	if signature == nil {
		return nil, ErrMissingBLSPrivateKey
	}
	return types.Signature(signature.Compress()), nil
}

func (adapter BLSTBLSAdapter) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	parsedPublicKey, err := parseBLSTBLSPublicKey(publicKey)
	if err != nil {
		return false
	}
	parsedSignature, err := parseBLSTBLSSignature(signature)
	if err != nil {
		return false
	}
	return parsedSignature.Verify(true, parsedPublicKey, true, blsMessage(message), []byte(blstBLSMessageDST), true)
}

func (adapter BLSTBLSAdapter) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	if len(signatures) == 0 {
		return nil, ErrInvalidBLSProof
	}
	parsedSignatures := make([]*blst.P2Affine, 0, len(signatures))
	for _, signature := range signatures {
		parsedSignature, err := parseBLSTBLSSignature(signature)
		if err != nil {
			return nil, err
		}
		parsedSignatures = append(parsedSignatures, parsedSignature)
	}
	aggregator := new(blst.P2Aggregate)
	if !aggregator.Aggregate(parsedSignatures, true) {
		return nil, ErrInvalidBLSProof
	}
	return types.AggregateSignature(aggregator.ToAffine().Compress()), nil
}

func (adapter BLSTBLSAdapter) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if len(publicKeys) == 0 {
		return false
	}
	parsedSignature, err := parseBLSTBLSSignature(types.Signature(signature))
	if err != nil {
		return false
	}
	parsedPublicKeys := make([]*blst.P1Affine, 0, len(publicKeys))
	seen := make(map[string]struct{}, len(publicKeys))
	for _, publicKey := range publicKeys {
		keyString := string(publicKey)
		if _, found := seen[keyString]; found {
			return false
		}
		seen[keyString] = struct{}{}
		parsedPublicKey, err := parseBLSTBLSPublicKey(publicKey)
		if err != nil {
			return false
		}
		parsedPublicKeys = append(parsedPublicKeys, parsedPublicKey)
	}
	return parsedSignature.FastAggregateVerify(true, parsedPublicKeys, blsMessage(message), []byte(blstBLSMessageDST), true)
}

func (adapter BLSTBLSAdapter) ValidatePublicKey(publicKey types.PublicKey) error {
	_, err := parseBLSTBLSPublicKey(publicKey)
	return err
}

func (adapter BLSTBLSAdapter) ProofOfPossession() (types.Signature, error) {
	publicKey := adapter.PublicKey()
	if len(publicKey) == 0 {
		return nil, ErrMissingBLSPrivateKey
	}
	return adapter.Sign(blsProofOfPossessionMessage(publicKey))
}

func (adapter BLSTBLSAdapter) VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool {
	return adapter.Verify(publicKey, blsProofOfPossessionMessage(publicKey), proof)
}

func (adapter BLSTBLSAdapter) Metadata() BLSAdapterMetadata {
	return BLSAdapterMetadata{
		Name:                  BLSAdapterBLSTName,
		Version:               blstBLSVersion,
		Audited:               true,
		AuditReport:           blstBLSAuditReportTag,
		DependencyAudit:       blstBLSDependencyTag,
		DomainSeparation:      true,
		PublicKeyValidation:   true,
		SubgroupChecks:        true,
		RogueKeyDefense:       true,
		DeterministicEncoding: true,
		MalformedInputFuzzed:  true,
		ProofOfPossession:     true,
	}
}

func parseBLSTBLSPublicKey(publicKey types.PublicKey) (*blst.P1Affine, error) {
	if len(publicKey) != blstBLSPublicKeyBytes {
		return nil, ErrInvalidBLSPublicKey
	}
	parsedPublicKey := new(blst.P1Affine)
	if parsedPublicKey.Uncompress(publicKey) == nil || !parsedPublicKey.KeyValidate() {
		return nil, ErrInvalidBLSPublicKey
	}
	return parsedPublicKey, nil
}

func parseBLSTBLSSignature(signature types.Signature) (*blst.P2Affine, error) {
	if len(signature) != blstBLSSignatureBytes {
		return nil, ErrInvalidBLSProof
	}
	parsedSignature := new(blst.P2Affine)
	if parsedSignature.Uncompress(signature) == nil || !parsedSignature.SigValidate(true) {
		return nil, ErrInvalidBLSProof
	}
	return parsedSignature, nil
}
