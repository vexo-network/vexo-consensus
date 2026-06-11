//go:build !cgo

package crypto

import "github.com/vexo-network/vexo-consensus/types"

const (
	BLSAdapterBLSTName    = "blst-bls12381-minpk-v1"
	blstBLSVersion        = "v1"
	blstBLSDependencyTag  = "github.com/supranational/blst@v0.3.16"
	blstBLSAuditReportTag = "ncc-group-blst-security-assessment"
)

type BLSTBLSAdapter struct{}

func GenerateBLSTBLSAdapter() (BLSTBLSAdapter, error) {
	return BLSTBLSAdapter{}, ErrBLSBackendUnavailable
}

func NewBLSTBLSAdapterFromSeed([]byte) (BLSTBLSAdapter, error) {
	return BLSTBLSAdapter{}, ErrBLSBackendUnavailable
}

func NewBLSTBLSAdapterFromPrivateKey([]byte) (BLSTBLSAdapter, error) {
	return BLSTBLSAdapter{}, ErrBLSBackendUnavailable
}

func NewBLSTBLSVerifierAdapter() BLSTBLSAdapter {
	return BLSTBLSAdapter{}
}

func NewBLSTBLSKeyDocument(BLSTBLSAdapter) (KeyDocument, error) {
	return KeyDocument{}, ErrBLSBackendUnavailable
}

func (document KeyDocument) BLSTBLSSigner() (BLSTBLSAdapter, error) {
	return BLSTBLSAdapter{}, ErrBLSBackendUnavailable
}

func (document KeyDocument) BLSTBLSSignerWithPassphrase(string) (BLSTBLSAdapter, error) {
	return BLSTBLSAdapter{}, ErrBLSBackendUnavailable
}

func (adapter BLSTBLSAdapter) PublicKey() types.PublicKey {
	return nil
}

func (adapter BLSTBLSAdapter) Sign([]byte) (types.Signature, error) {
	return nil, ErrBLSBackendUnavailable
}

func (adapter BLSTBLSAdapter) Verify(types.PublicKey, []byte, types.Signature) bool {
	return false
}

func (adapter BLSTBLSAdapter) Aggregate([]types.Signature) (types.AggregateSignature, error) {
	return nil, ErrBLSBackendUnavailable
}

func (adapter BLSTBLSAdapter) VerifyAggregate([]types.PublicKey, []byte, types.AggregateSignature) bool {
	return false
}

func (adapter BLSTBLSAdapter) ValidatePublicKey(types.PublicKey) error {
	return ErrBLSBackendUnavailable
}

func (adapter BLSTBLSAdapter) ProofOfPossession() (types.Signature, error) {
	return nil, ErrBLSBackendUnavailable
}

func (adapter BLSTBLSAdapter) VerifyProofOfPossession(types.PublicKey, types.Signature) bool {
	return false
}

func (adapter BLSTBLSAdapter) Metadata() BLSAdapterMetadata {
	return BLSAdapterMetadata{
		Name:            BLSAdapterBLSTName,
		Version:         blstBLSVersion,
		AuditReport:     blstBLSAuditReportTag,
		DependencyAudit: blstBLSDependencyTag,
	}
}
