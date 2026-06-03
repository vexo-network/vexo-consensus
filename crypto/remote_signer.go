package crypto

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingRemoteSignerURL = errors.New("remote signer url is required")
	ErrMissingRemotePublicKey = errors.New("remote signer public key is required")
	ErrRemoteSignerRejected   = errors.New("remote signer rejected request")
	ErrMissingSignPolicy      = errors.New("remote signer sign policy is required")
	ErrInvalidSignPolicy      = errors.New("remote signer sign policy is invalid")
	ErrDoubleSign             = errors.New("remote signer double-sign guard rejected conflicting request")
)

type SignType string

const (
	SignTypeConsensusProposal    SignType = "consensus_proposal"
	SignTypeConsensusVote        SignType = "consensus_vote"
	SignTypeConsensusTimeoutVote SignType = "consensus_timeout_vote"
	SignTypeFinalityProof        SignType = "finality_proof"
)

type SignPolicy struct {
	ChainID string       `json:"chain_id"`
	Height  types.Height `json:"height"`
	Round   types.Round  `json:"round"`
	Type    SignType     `json:"type"`
	Domain  Domain       `json:"domain"`
}

type DoubleSignGuard struct {
	seen map[string][32]byte
}

type RemoteSigner struct {
	url       string
	publicKey types.PublicKey
	client    *http.Client
	verifier  Signer
	guard     *DoubleSignGuard
}

type remoteSignRequest struct {
	PublicKey string      `json:"public_key"`
	Message   string      `json:"message"`
	Policy    *SignPolicy `json:"policy,omitempty"`
}

type remoteSignResponse struct {
	Signature string `json:"signature"`
}

func NewRemoteSigner(url string, publicKey types.PublicKey, verifier Signer, timeout time.Duration) (RemoteSigner, error) {
	return NewRemoteSignerWithGuard(url, publicKey, verifier, timeout, nil)
}

func NewRemoteSignerWithGuard(url string, publicKey types.PublicKey, verifier Signer, timeout time.Duration, guard *DoubleSignGuard) (RemoteSigner, error) {
	if url == "" {
		return RemoteSigner{}, ErrMissingRemoteSignerURL
	}
	if len(publicKey) == 0 {
		return RemoteSigner{}, ErrMissingRemotePublicKey
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return RemoteSigner{
		url:       url,
		publicKey: append(types.PublicKey(nil), publicKey...),
		client:    &http.Client{Timeout: timeout},
		verifier:  verifier,
		guard:     guard,
	}, nil
}

func (signer RemoteSigner) PublicKey() types.PublicKey {
	return append(types.PublicKey(nil), signer.publicKey...)
}

func (signer RemoteSigner) Sign(message []byte) (types.Signature, error) {
	return signer.signWithPolicy(message, nil)
}

func (signer RemoteSigner) SignWithPolicy(policy SignPolicy, message []byte) (types.Signature, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if signer.guard != nil {
		if err := signer.guard.CheckAndRemember(policy, message); err != nil {
			return nil, err
		}
	}
	return signer.signWithPolicy(message, &policy)
}

func (signer RemoteSigner) signWithPolicy(message []byte, policy *SignPolicy) (types.Signature, error) {
	payload := remoteSignRequest{
		PublicKey: base64.StdEncoding.EncodeToString(signer.publicKey),
		Message:   base64.StdEncoding.EncodeToString(message),
		Policy:    policy,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, signer.url, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := signer.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status=%d", ErrRemoteSignerRejected, response.StatusCode)
	}
	var payloadResponse remoteSignResponse
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payloadResponse); err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(payloadResponse.Signature)
	if err != nil {
		return nil, err
	}
	return types.Signature(signature), nil
}

func (signer RemoteSigner) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	if signer.verifier == nil {
		return false
	}
	return signer.verifier.Verify(publicKey, message, signature)
}

func (policy SignPolicy) Validate() error {
	if policy.ChainID == "" || policy.Height == 0 || policy.Type == "" || policy.Domain == "" {
		return ErrMissingSignPolicy
	}
	expected, ok := signTypeDomains[policy.Type]
	if !ok || expected != policy.Domain {
		return ErrInvalidSignPolicy
	}
	return nil
}

var signTypeDomains = map[SignType]Domain{
	SignTypeConsensusProposal:    DomainConsensusProposal,
	SignTypeConsensusVote:        DomainConsensusVote,
	SignTypeConsensusTimeoutVote: DomainConsensusTimeoutVote,
	SignTypeFinalityProof:        DomainFinalityProof,
}

func NewDoubleSignGuard() *DoubleSignGuard {
	return &DoubleSignGuard{seen: make(map[string][32]byte)}
}

func NewDoubleSignGuardFromSnapshot(snapshot map[string]string) (*DoubleSignGuard, error) {
	guard := NewDoubleSignGuard()
	for key, encodedDigest := range snapshot {
		digest, err := base64.StdEncoding.DecodeString(encodedDigest)
		if err != nil {
			return nil, err
		}
		if len(digest) != sha256.Size {
			return nil, ErrDoubleSign
		}
		var fixed [32]byte
		copy(fixed[:], digest)
		guard.seen[key] = fixed
	}
	return guard, nil
}

func (guard *DoubleSignGuard) CheckAndRemember(policy SignPolicy, message []byte) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if guard.seen == nil {
		guard.seen = make(map[string][32]byte)
	}
	key := policy.GuardKey()
	digest := sha256.Sum256(message)
	if previous, found := guard.seen[key]; found && previous != digest {
		return ErrDoubleSign
	}
	guard.seen[key] = digest
	return nil
}

func (guard *DoubleSignGuard) Snapshot() map[string]string {
	snapshot := make(map[string]string, len(guard.seen))
	for key, digest := range guard.seen {
		snapshot[key] = base64.StdEncoding.EncodeToString(digest[:])
	}
	return snapshot
}

func (policy SignPolicy) GuardKey() string {
	return fmt.Sprintf("%s/%d/%d/%s/%s", policy.ChainID, policy.Height, policy.Round, policy.Type, policy.Domain)
}
