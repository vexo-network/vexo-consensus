package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	ErrRemoteSignatureInvalid  = errors.New("remote signer returned invalid signature")
	ErrMissingGuardPath        = errors.New("double-sign guard path is required")
	ErrMissingNonceGuardPath   = errors.New("remote signer nonce guard path is required")
	ErrRemoteNonceUnavailable  = errors.New("remote signer nonce entropy is unavailable")
)

type SignType string

const (
	SignTypeConsensusProposal    SignType = "consensus_proposal"
	SignTypeConsensusVote        SignType = "consensus_vote"
	SignTypeConsensusTimeoutVote SignType = "consensus_timeout_vote"
	SignTypeRemoteSignerProbe    SignType = "remote_signer_probe"
	SignTypeLaneBatch            SignType = "lane_batch"
	SignTypeLaneBatchAck         SignType = "lane_batch_ack"
)

type SignPolicy struct {
	ChainID string       `json:"chain_id"`
	Height  types.Height `json:"height"`
	Round   types.Round  `json:"round"`
	Type    SignType     `json:"type"`
	Domain  Domain       `json:"domain"`
}

// DoubleSignGuard refuses to sign a consensus statement that conflicts with, or falls
// behind, what a key has already signed. It keeps one high-water mark per public key and
// statement kind: the highest view signed and the digest of the bytes signed at that
// view. The state is bounded by the number of keys, and persisted atomically when a mark
// advances.
type DoubleSignGuard struct {
	mu    sync.Mutex
	marks map[string]map[SignType]highWaterMark
	path  string
}

type highWaterMark struct {
	view   types.Round
	digest [sha256.Size]byte
}

type RemoteSignerNonceGuard struct {
	mu           sync.Mutex
	seen         map[string]struct{}
	order        []string
	snapshotPath string
	maxNonces    int
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
	nonceGuard *RemoteSignerNonceGuard
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

// doubleSignGuardSchema names the high-water-mark state format. State written by the
// earlier per-message record format carries another schema and is refused at load.
const doubleSignGuardSchema = "high-water-mark/v1"

type doubleSignGuardDocument struct {
	SchemaVersion  string                                  `json:"schema_version"`
	HighWaterMarks map[string]map[SignType]guardMarkRecord `json:"high_water_marks"`
}

type guardMarkRecord struct {
	View   types.Round `json:"view"`
	Digest string      `json:"digest"`
}

type remoteSignerNonceSnapshotDocument struct {
	SchemaVersion string   `json:"schema_version"`
	Nonces        []string `json:"nonces"`
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
	ctx, cancel := signer.defaultContext()
	defer cancel()
	return signer.SignWithContext(ctx, message)
}

func (signer RemoteSigner) SignWithContext(ctx context.Context, message []byte) (types.Signature, error) {
	if signer.requirePolicy {
		return nil, ErrMissingSignPolicy
	}
	if err := signer.checkGuard(message); err != nil {
		return nil, err
	}
	return signer.signWithPolicy(ctx, message, nil)
}

func (signer RemoteSigner) SignWithPolicy(policy SignPolicy, message []byte) (types.Signature, error) {
	ctx, cancel := signer.defaultContext()
	defer cancel()
	return signer.SignWithPolicyContext(ctx, policy, message)
}

func (signer RemoteSigner) SignWithPolicyContext(ctx context.Context, policy SignPolicy, message []byte) (types.Signature, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := policy.Matches(message); err != nil {
		return nil, err
	}
	if err := signer.checkGuard(message); err != nil {
		return nil, err
	}
	return signer.signWithPolicy(ctx, message, &policy)
}

// checkGuard applies the double-sign guard to the bytes about to be signed, whether or not
// the caller declared a policy.
func (signer RemoteSigner) checkGuard(message []byte) error {
	if signer.guard == nil {
		return nil
	}
	return signer.guard.CheckAndRemember(signer.publicKey, message)
}

func (signer RemoteSigner) defaultContext() (context.Context, context.CancelFunc) {
	timeout := 5 * time.Second
	if signer.client != nil && signer.client.Timeout > 0 {
		timeout = signer.client.Timeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (signer RemoteSigner) signWithPolicy(ctx context.Context, message []byte, policy *SignPolicy) (types.Signature, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	nonce, err := newRemoteSignerNonce()
	if err != nil {
		return nil, err
	}
	payload := remoteSignRequest{
		PublicKey: base64.StdEncoding.EncodeToString(signer.publicKey),
		Message:   base64.StdEncoding.EncodeToString(message),
		Policy:    policy,
		Nonce:     nonce,
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
	typedSignature := types.Signature(signature)
	if signer.verifier != nil && !signer.verifier.Verify(signer.publicKey, message, typedSignature) {
		return nil, ErrRemoteSignatureInvalid
	}
	return typedSignature, nil
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
	kind, ok := signKindOf(policy.Type)
	if !ok || kind.domain != policy.Domain {
		return ErrInvalidSignPolicy
	}
	return nil
}

func NewDoubleSignGuard() *DoubleSignGuard {
	return &DoubleSignGuard{marks: make(map[string]map[SignType]highWaterMark)}
}

func NewFileBackedDoubleSignGuard(path string) (*DoubleSignGuard, error) {
	if path == "" {
		return nil, ErrMissingGuardPath
	}
	guard, err := loadDoubleSignGuard(path)
	if errors.Is(err, os.ErrNotExist) {
		guard = NewDoubleSignGuard()
		guard.path = path
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return guard, nil
}

// CheckAndRemember decides from message alone whether publicKey may sign it. The
// statement kind, height and view are read from the bytes, never from a declared policy.
// A consensus statement is signed only when its view is above the key's high-water mark
// for that kind, or when it is byte-identical to the statement already signed at the mark.
// Anything at or below the mark that differs, including every older view, is refused: the
// guard keeps no history below the mark, so it cannot show that an older statement does not
// conflict. The mark is durable before CheckAndRemember returns nil.
func (guard *DoubleSignGuard) CheckAndRemember(publicKey types.PublicKey, message []byte) error {
	statement, err := parseSignMessage(message)
	if err != nil {
		return err
	}
	if !statement.guarded {
		return nil
	}
	if len(publicKey) == 0 {
		return ErrMissingRemotePublicKey
	}
	holder := base64.StdEncoding.EncodeToString(publicKey)
	next := highWaterMark{view: statement.view, digest: sha256.Sum256(message)}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	previous, found := guard.marks[holder][statement.signType]
	if found {
		switch {
		case next.view < previous.view:
			return fmt.Errorf("%w: %s view %d is below the high-water mark %d", ErrDoubleSign, statement.signType, next.view, previous.view)
		case next.view == previous.view && next.digest != previous.digest:
			return fmt.Errorf("%w: conflicting %s at view %d", ErrDoubleSign, statement.signType, next.view)
		case next.view == previous.view:
			return nil
		}
	}
	if guard.marks == nil {
		guard.marks = make(map[string]map[SignType]highWaterMark)
	}
	if guard.marks[holder] == nil {
		guard.marks[holder] = make(map[SignType]highWaterMark)
	}
	guard.marks[holder][statement.signType] = next
	if guard.path == "" {
		return nil
	}
	if err := guard.saveLocked(); err != nil {
		if found {
			guard.marks[holder][statement.signType] = previous
		} else {
			delete(guard.marks[holder], statement.signType)
			if len(guard.marks[holder]) == 0 {
				delete(guard.marks, holder)
			}
		}
		return err
	}
	return nil
}

func loadDoubleSignGuard(path string) (*DoubleSignGuard, error) {
	if path == "" {
		return nil, ErrMissingGuardPath
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(encoded, &header); err != nil {
		return nil, err
	}
	if header.SchemaVersion != doubleSignGuardSchema {
		return nil, fmt.Errorf("%w: double-sign guard state schema %q is not %q; it was written by an older signer, so confirm the last view the key signed before replacing it",
			ErrUnsupportedKeyVersion, header.SchemaVersion, doubleSignGuardSchema)
	}
	var document doubleSignGuardDocument
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	guard := NewDoubleSignGuard()
	guard.path = path
	for holder, records := range document.HighWaterMarks {
		publicKey, err := base64.StdEncoding.DecodeString(holder)
		if err != nil || len(publicKey) == 0 {
			return nil, fmt.Errorf("%w: invalid public key %q", ErrDoubleSign, holder)
		}
		marks := make(map[SignType]highWaterMark, len(records))
		for signType, record := range records {
			if kind, known := signKindOf(signType); !known || !kind.guarded() {
				return nil, fmt.Errorf("%w: unsupported statement kind %q", ErrDoubleSign, signType)
			}
			digest, err := base64.StdEncoding.DecodeString(record.Digest)
			if err != nil || len(digest) != sha256.Size {
				return nil, fmt.Errorf("%w: invalid digest for %s", ErrDoubleSign, signType)
			}
			mark := highWaterMark{view: record.View}
			copy(mark.digest[:], digest)
			marks[signType] = mark
		}
		guard.marks[holder] = marks
	}
	return guard, nil
}

func (guard *DoubleSignGuard) saveLocked() error {
	document := doubleSignGuardDocument{
		SchemaVersion:  doubleSignGuardSchema,
		HighWaterMarks: make(map[string]map[SignType]guardMarkRecord, len(guard.marks)),
	}
	for holder, marks := range guard.marks {
		records := make(map[SignType]guardMarkRecord, len(marks))
		for signType, mark := range marks {
			records[signType] = guardMarkRecord{View: mark.view, Digest: base64.StdEncoding.EncodeToString(mark.digest[:])}
		}
		document.HighWaterMarks[holder] = records
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(guard.path, append(encoded, '\n'))
}

func NewRemoteSignerNonceGuard() *RemoteSignerNonceGuard {
	return &RemoteSignerNonceGuard{seen: make(map[string]struct{}), maxNonces: 4096}
}

func NewFileBackedRemoteSignerNonceGuard(path string) (*RemoteSignerNonceGuard, error) {
	if path == "" {
		return nil, ErrMissingNonceGuardPath
	}
	guard, err := LoadRemoteSignerNonceGuard(path)
	if errors.Is(err, os.ErrNotExist) {
		guard = NewRemoteSignerNonceGuard()
		err = nil
	}
	if err != nil {
		return nil, err
	}
	guard.snapshotPath = path
	if guard.maxNonces <= 0 {
		guard.maxNonces = 4096
	}
	return guard, nil
}

func LoadRemoteSignerNonceGuard(path string) (*RemoteSignerNonceGuard, error) {
	if path == "" {
		return nil, ErrMissingNonceGuardPath
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var document remoteSignerNonceSnapshotDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return nil, ErrUnsupportedKeyVersion
	}
	guard := NewRemoteSignerNonceGuard()
	for _, nonce := range document.Nonces {
		if nonce == "" {
			continue
		}
		if _, found := guard.seen[nonce]; found {
			continue
		}
		guard.seen[nonce] = struct{}{}
		guard.order = append(guard.order, nonce)
	}
	guard.snapshotPath = path
	guard.evictLocked()
	return guard, nil
}

func (guard *RemoteSignerNonceGuard) Remember(nonce string) (bool, error) {
	if nonce == "" {
		return true, nil
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.seen == nil {
		guard.seen = make(map[string]struct{})
	}
	if guard.maxNonces <= 0 {
		guard.maxNonces = 4096
	}
	if _, found := guard.seen[nonce]; found {
		return true, nil
	}
	guard.seen[nonce] = struct{}{}
	guard.order = append(guard.order, nonce)
	guard.evictLocked()
	if guard.snapshotPath != "" {
		return false, guard.saveLocked(guard.snapshotPath)
	}
	return false, nil
}

func (guard *RemoteSignerNonceGuard) evictLocked() {
	for guard.maxNonces > 0 && len(guard.order) > guard.maxNonces {
		oldest := guard.order[0]
		copy(guard.order, guard.order[1:])
		guard.order = guard.order[:len(guard.order)-1]
		delete(guard.seen, oldest)
	}
}

func (guard *RemoteSignerNonceGuard) saveLocked(path string) error {
	if path == "" {
		return ErrMissingNonceGuardPath
	}
	document := remoteSignerNonceSnapshotDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Nonces:        append([]string(nil), guard.order...),
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(encoded, '\n'))
}

// writeFileAtomic replaces path with data so that a crash leaves either the old or the new
// content, never a mixture: it writes and syncs a private temporary file in the same
// directory, renames it over path, and syncs the directory so the rename is durable.
func writeFileAtomic(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	dirFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}

func NewRemoteSignerService(signer Signer, policy RemoteSignerPolicy, guard *DoubleSignGuard) (*RemoteSignerService, error) {
	return NewRemoteSignerServiceWithNonceGuard(signer, policy, guard, nil)
}

func NewRemoteSignerServiceWithNonceGuard(signer Signer, policy RemoteSignerPolicy, guard *DoubleSignGuard, nonceGuard *RemoteSignerNonceGuard) (*RemoteSignerService, error) {
	if signer == nil {
		return nil, ErrMissingKMSSigner
	}
	if guard == nil {
		guard = NewDoubleSignGuard()
	}
	if nonceGuard == nil {
		nonceGuard = NewRemoteSignerNonceGuard()
	}
	if policy.RequirePolicy || policy.ChainID != "" || len(policy.AllowedTypes) > 0 || policy.MinHeight > 0 || policy.MaxHeight > 0 {
		policy.RequirePolicy = true
	}
	return &RemoteSignerService{signer: signer, policy: policy, guard: guard, nonceGuard: nonceGuard}, nil
}

func (service *RemoteSignerService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if service.policy.AuthToken != "" {
		if !remoteSignerBearerAuthorized(request.Header.Get("Authorization"), service.policy.AuthToken) {
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
		replayed, err := service.seenNonce(payload.Nonce)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if replayed {
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
	if err := service.policy.Validate(payload.Policy, message); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	if err := service.guard.CheckAndRemember(service.signer.PublicKey(), message); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	signature, err := service.signer.Sign(message)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
}

func (service *RemoteSignerService) seenNonce(nonce string) (bool, error) {
	if service.nonceGuard == nil {
		service.nonceGuard = NewRemoteSignerNonceGuard()
	}
	return service.nonceGuard.Remember(nonce)
}

func newRemoteSignerNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", ErrRemoteNonceUnavailable
	}
	return hex.EncodeToString(nonce[:]), nil
}

func remoteSignerBearerAuthorized(header string, token string) bool {
	expected := "Bearer " + token
	headerDigest := sha256.Sum256([]byte(header))
	expectedDigest := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(headerDigest[:], expectedDigest[:]) == 1
}

// Validate checks the policy a request declares against the service configuration and
// against message, the bytes the service is about to sign.
func (policy RemoteSignerPolicy) Validate(signPolicy *SignPolicy, message []byte) error {
	if signPolicy == nil {
		if policy.RequirePolicy {
			return ErrMissingSignPolicy
		}
		return nil
	}
	if err := signPolicy.Validate(); err != nil {
		return err
	}
	if err := signPolicy.Matches(message); err != nil {
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
