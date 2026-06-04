package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingRemoteSignerURL  = errors.New("remote signer url is required")
	ErrMissingRemotePublicKey  = errors.New("remote signer public key is required")
	ErrRemoteSignerRejected    = errors.New("remote signer rejected request")
	ErrMissingSignPolicy       = errors.New("remote signer sign policy is required")
	ErrInvalidSignPolicy       = errors.New("remote signer sign policy is invalid")
	ErrDoubleSign              = errors.New("remote signer double-sign guard rejected conflicting request")
	ErrMissingKMSSigner        = errors.New("kms signer is required")
	ErrRemotePublicKeyMismatch = errors.New("remote signer public key mismatch")
	ErrMissingGuardPath        = errors.New("double-sign guard path is required")
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
	mu           sync.Mutex
	seen         map[string][32]byte
	snapshotPath string
}

type RemoteSignerPolicy struct {
	ChainID       string
	MinHeight     types.Height
	MaxHeight     types.Height
	AllowedTypes  []SignType
	RequirePolicy bool
	AuthToken     string
}

type RemoteSignerService struct {
	signer     typesSigner
	policy     RemoteSignerPolicy
	guard      *DoubleSignGuard
	mu         sync.Mutex
	seenNonces map[string]struct{}
	nonceOrder []string
}

type typesSigner interface {
	Signer
}

type RemoteSigner struct {
	url           string
	publicKey     types.PublicKey
	client        *http.Client
	verifier      Signer
	guard         *DoubleSignGuard
	authToken     string
	requirePolicy bool
}

type remoteSignRequest struct {
	PublicKey string      `json:"public_key"`
	Message   string      `json:"message"`
	Policy    *SignPolicy `json:"policy,omitempty"`
	Nonce     string      `json:"nonce,omitempty"`
}

type remoteSignResponse struct {
	Signature string `json:"signature"`
}

type doubleSignGuardSnapshotDocument struct {
	SchemaVersion string            `json:"schema_version"`
	Records       map[string]string `json:"records"`
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

func NewRemoteSignerWithAuth(url string, publicKey types.PublicKey, verifier Signer, timeout time.Duration, guard *DoubleSignGuard, authToken string) (RemoteSigner, error) {
	signer, err := NewRemoteSignerWithGuard(url, publicKey, verifier, timeout, guard)
	if err != nil {
		return RemoteSigner{}, err
	}
	signer.authToken = authToken
	return signer, nil
}

func (signer RemoteSigner) PublicKey() types.PublicKey {
	return append(types.PublicKey(nil), signer.publicKey...)
}

func (signer RemoteSigner) Sign(message []byte) (types.Signature, error) {
	return signer.SignWithContext(context.Background(), message)
}

func (signer RemoteSigner) SignWithContext(ctx context.Context, message []byte) (types.Signature, error) {
	return signer.signWithPolicy(ctx, message, nil)
}

func (signer RemoteSigner) SignWithPolicy(policy SignPolicy, message []byte) (types.Signature, error) {
	return signer.SignWithPolicyContext(context.Background(), policy, message)
}

func (signer RemoteSigner) SignWithPolicyContext(ctx context.Context, policy SignPolicy, message []byte) (types.Signature, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if signer.guard != nil {
		if err := signer.guard.CheckAndRemember(policy, message); err != nil {
			return nil, err
		}
	}
	return signer.signWithPolicy(ctx, message, &policy)
}

func (signer RemoteSigner) signWithPolicy(ctx context.Context, message []byte, policy *SignPolicy) (types.Signature, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if signer.requirePolicy && policy == nil {
		return nil, ErrMissingSignPolicy
	}
	payload := remoteSignRequest{
		PublicKey: base64.StdEncoding.EncodeToString(signer.publicKey),
		Message:   base64.StdEncoding.EncodeToString(message),
		Policy:    policy,
		Nonce:     newRemoteSignerNonce(),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, signer.url, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if signer.authToken != "" {
		request.Header.Set("Authorization", "Bearer "+signer.authToken)
	}
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

func NewFileBackedDoubleSignGuard(path string) (*DoubleSignGuard, error) {
	if path == "" {
		return nil, ErrMissingGuardPath
	}
	guard, err := LoadDoubleSignGuard(path)
	if errors.Is(err, os.ErrNotExist) {
		guard = NewDoubleSignGuard()
		err = nil
	}
	if err != nil {
		return nil, err
	}
	guard.snapshotPath = path
	return guard, nil
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
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.seen == nil {
		guard.seen = make(map[string][32]byte)
	}
	key := policy.GuardKey()
	digest := sha256.Sum256(message)
	if previous, found := guard.seen[key]; found && previous != digest {
		return ErrDoubleSign
	}
	guard.seen[key] = digest
	if guard.snapshotPath != "" {
		return guard.saveLocked(guard.snapshotPath)
	}
	return nil
}

func (guard *DoubleSignGuard) Snapshot() map[string]string {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	snapshot := make(map[string]string, len(guard.seen))
	for key, digest := range guard.seen {
		snapshot[key] = base64.StdEncoding.EncodeToString(digest[:])
	}
	return snapshot
}

func (policy SignPolicy) GuardKey() string {
	return fmt.Sprintf("%s/%d/%d/%s/%s", policy.ChainID, policy.Height, policy.Round, policy.Type, policy.Domain)
}

func SaveDoubleSignGuard(path string, guard *DoubleSignGuard) error {
	if path == "" {
		return ErrMissingGuardPath
	}
	if guard == nil {
		guard = NewDoubleSignGuard()
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	return guard.saveLocked(path)
}

func LoadDoubleSignGuard(path string) (*DoubleSignGuard, error) {
	if path == "" {
		return nil, ErrMissingGuardPath
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var document doubleSignGuardSnapshotDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return nil, ErrUnsupportedKeyVersion
	}
	guard, err := NewDoubleSignGuardFromSnapshot(document.Records)
	if err != nil {
		return nil, err
	}
	guard.snapshotPath = path
	return guard, nil
}

func (guard *DoubleSignGuard) saveLocked(path string) error {
	if path == "" {
		return ErrMissingGuardPath
	}
	records := make(map[string]string, len(guard.seen))
	for key, digest := range guard.seen {
		records[key] = base64.StdEncoding.EncodeToString(digest[:])
	}
	document := doubleSignGuardSnapshotDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Records:       records,
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tempPath := path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	if dirFile, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

func NewRemoteSignerService(signer Signer, policy RemoteSignerPolicy, guard *DoubleSignGuard) (*RemoteSignerService, error) {
	if signer == nil {
		return nil, ErrMissingKMSSigner
	}
	if guard == nil {
		guard = NewDoubleSignGuard()
	}
	if policy.RequirePolicy || policy.ChainID != "" || len(policy.AllowedTypes) > 0 || policy.MinHeight > 0 || policy.MaxHeight > 0 {
		policy.RequirePolicy = true
	}
	return &RemoteSignerService{signer: signer, policy: policy, guard: guard, seenNonces: make(map[string]struct{})}, nil
}

func (service *RemoteSignerService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if service.policy.AuthToken != "" {
		if request.Header.Get("Authorization") != "Bearer "+service.policy.AuthToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	var payload remoteSignRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if service.policy.AuthToken != "" {
		if payload.Nonce == "" || service.seenNonce(payload.Nonce) {
			http.Error(writer, "remote signer replay nonce rejected", http.StatusForbidden)
			return
		}
	}
	publicKey, err := base64.StdEncoding.DecodeString(payload.PublicKey)
	if err != nil || !bytes.Equal(publicKey, service.signer.PublicKey()) {
		http.Error(writer, ErrRemotePublicKeyMismatch.Error(), http.StatusForbidden)
		return
	}
	message, err := base64.StdEncoding.DecodeString(payload.Message)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := service.policy.Validate(payload.Policy); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	if payload.Policy != nil {
		if err := service.guard.CheckAndRemember(*payload.Policy, message); err != nil {
			http.Error(writer, err.Error(), http.StatusForbidden)
			return
		}
	}
	signature, err := service.signer.Sign(message)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
}

func (service *RemoteSignerService) seenNonce(nonce string) bool {
	const maxSeenNonces = 4096
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, found := service.seenNonces[nonce]; found {
		return true
	}
	service.seenNonces[nonce] = struct{}{}
	service.nonceOrder = append(service.nonceOrder, nonce)
	if len(service.nonceOrder) > maxSeenNonces {
		oldest := service.nonceOrder[0]
		copy(service.nonceOrder, service.nonceOrder[1:])
		service.nonceOrder = service.nonceOrder[:len(service.nonceOrder)-1]
		delete(service.seenNonces, oldest)
	}
	return false
}

func newRemoteSignerNonce() string {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(nonce[:])
}

func (policy RemoteSignerPolicy) Validate(signPolicy *SignPolicy) error {
	if signPolicy == nil {
		if policy.RequirePolicy {
			return ErrMissingSignPolicy
		}
		return nil
	}
	if err := signPolicy.Validate(); err != nil {
		return err
	}
	if policy.ChainID != "" && signPolicy.ChainID != policy.ChainID {
		return ErrInvalidSignPolicy
	}
	if policy.MinHeight > 0 && signPolicy.Height < policy.MinHeight {
		return ErrInvalidSignPolicy
	}
	if policy.MaxHeight > 0 && signPolicy.Height > policy.MaxHeight {
		return ErrInvalidSignPolicy
	}
	if len(policy.AllowedTypes) > 0 {
		for _, allowedType := range policy.AllowedTypes {
			if signPolicy.Type == allowedType {
				return nil
			}
		}
		return ErrInvalidSignPolicy
	}
	return nil
}
