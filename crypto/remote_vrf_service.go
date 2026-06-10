package crypto

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

const (
	remoteVRFProveDomain  = "vexo.remote_vrf.prove.v1"
	remoteVRFVerifyDomain = "vexo.remote_vrf.verify.v1"
)

var (
	ErrRemoteVRFUnauthorized     = errors.New("remote vrf request is unauthorized")
	ErrRemoteVRFInvalidChallenge = errors.New("remote vrf challenge is invalid")
	ErrRemoteVRFDuplicateNonce   = errors.New("remote vrf nonce was already used")
	ErrRemoteVRFReplayStore      = errors.New("remote vrf replay store error")
)

type RemoteVRFServiceConfig struct {
	AuthToken                 string
	MaxSkew                   time.Duration
	NonceTTL                  time.Duration
	ReplayStore               RemoteVRFReplayStore
	RequireDurableReplayStore bool
	AuditSink                 func(RemoteVRFAuditEvent)
}

type RemoteVRFAuditEvent struct {
	Path       string
	RemoteAddr string
	Authorized bool
	Reason     string
	Nonce      string
	At         time.Time
}

type RemoteVRFReplayStore interface {
	MarkNonce(domain string, nonce string, expires time.Time, now time.Time) error
}

type RemoteVRFService struct {
	vrf      VRF
	cfg      RemoteVRFServiceConfig
	mu       sync.Mutex
	seen     map[string]time.Time
	now      func() time.Time
	maxDelay time.Duration
}

func NewRemoteVRFService(vrf VRF, cfg RemoteVRFServiceConfig) (*RemoteVRFService, error) {
	if vrf == nil {
		return nil, ErrVRFBackendUnavailable
	}
	if cfg.RequireDurableReplayStore && cfg.ReplayStore == nil {
		return nil, ErrRemoteVRFReplayStore
	}
	if cfg.MaxSkew <= 0 {
		cfg.MaxSkew = 30 * time.Second
	}
	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = 10 * time.Minute
	}
	return &RemoteVRFService{
		vrf:      vrf,
		cfg:      cfg,
		seen:     make(map[string]time.Time),
		now:      time.Now,
		maxDelay: cfg.MaxSkew,
	}, nil
}

func (service *RemoteVRFService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeRemoteVRFError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !service.authorized(request) {
		service.audit(request, false, "unauthorized", "")
		writeRemoteVRFError(writer, http.StatusUnauthorized, ErrRemoteVRFUnauthorized.Error())
		return
	}
	switch request.URL.Path {
	case "/prove":
		service.handleProve(writer, request)
	case "/verify":
		service.handleVerify(writer, request)
	default:
		writeRemoteVRFError(writer, http.StatusNotFound, "remote vrf endpoint not found")
	}
}

func (service *RemoteVRFService) handleProve(writer http.ResponseWriter, request *http.Request) {
	var payload remoteVRFProveRequest
	if err := decodeRemoteVRFJSON(request, &payload); err != nil {
		service.audit(request, true, err.Error(), "")
		writeRemoteVRFError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.validateChallenge(payload.Nonce, payload.IssuedAtUnixNano, payload.DeadlineUnixNano, payload.Domain, remoteVRFProveDomain); err != nil {
		service.audit(request, true, err.Error(), payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, err.Error())
		return
	}
	publicKey, err := base64.StdEncoding.DecodeString(payload.PublicKey)
	if err != nil {
		service.audit(request, true, "invalid public key", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid public key")
		return
	}
	seed, err := base64.StdEncoding.DecodeString(payload.Seed)
	if err != nil {
		service.audit(request, true, "invalid seed", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid seed")
		return
	}
	output, proof, err := service.vrf.Prove(types.PublicKey(publicKey), seed)
	if err != nil {
		service.audit(request, true, err.Error(), payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, err.Error())
		return
	}
	service.audit(request, true, "proved", payload.Nonce)
	writeRemoteVRFJSON(writer, http.StatusOK, remoteVRFProveResponse{
		Output: base64.StdEncoding.EncodeToString(output),
		Proof:  base64.StdEncoding.EncodeToString(proof),
		Nonce:  payload.Nonce,
	})
}

func (service *RemoteVRFService) handleVerify(writer http.ResponseWriter, request *http.Request) {
	var payload remoteVRFVerifyRequest
	if err := decodeRemoteVRFJSON(request, &payload); err != nil {
		service.audit(request, true, err.Error(), "")
		writeRemoteVRFError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.validateChallenge(payload.Nonce, payload.IssuedAtUnixNano, payload.DeadlineUnixNano, payload.Domain, remoteVRFVerifyDomain); err != nil {
		service.audit(request, true, err.Error(), payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, err.Error())
		return
	}
	publicKey, err := base64.StdEncoding.DecodeString(payload.PublicKey)
	if err != nil {
		service.audit(request, true, "invalid public key", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid public key")
		return
	}
	seed, err := base64.StdEncoding.DecodeString(payload.Seed)
	if err != nil {
		service.audit(request, true, "invalid seed", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid seed")
		return
	}
	output, err := base64.StdEncoding.DecodeString(payload.Output)
	if err != nil {
		service.audit(request, true, "invalid output", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid output")
		return
	}
	proof, err := base64.StdEncoding.DecodeString(payload.Proof)
	if err != nil {
		service.audit(request, true, "invalid proof", payload.Nonce)
		writeRemoteVRFError(writer, http.StatusBadRequest, "invalid proof")
		return
	}
	valid := service.vrf.Verify(types.PublicKey(publicKey), seed, output, proof)
	service.audit(request, true, "verified", payload.Nonce)
	writeRemoteVRFJSON(writer, http.StatusOK, remoteVRFVerifyResponse{Valid: valid, Nonce: payload.Nonce})
}

func (service *RemoteVRFService) authorized(request *http.Request) bool {
	if service.cfg.AuthToken == "" {
		return true
	}
	return request.Header.Get("Authorization") == "Bearer "+service.cfg.AuthToken
}

func (service *RemoteVRFService) validateChallenge(nonce string, issuedAt int64, deadline int64, actualDomain string, expectedDomain string) error {
	if nonce == "" || actualDomain != expectedDomain || issuedAt <= 0 || deadline <= issuedAt {
		return ErrRemoteVRFInvalidChallenge
	}
	now := service.now()
	issued := time.Unix(0, issuedAt)
	expires := time.Unix(0, deadline)
	if now.Add(service.maxDelay).Before(issued) || now.After(expires) || expires.Sub(issued) > service.maxDelay {
		return ErrRemoteVRFInvalidChallenge
	}
	return service.markNonce(actualDomain, nonce, expires)
}

func (service *RemoteVRFService) markNonce(domain string, nonce string, expires time.Time) error {
	now := service.now()
	if expires.After(now.Add(service.cfg.NonceTTL)) {
		expires = now.Add(service.cfg.NonceTTL)
	}
	if service.cfg.ReplayStore != nil {
		return service.cfg.ReplayStore.MarkNonce(domain, nonce, expires, now)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	for key, expiry := range service.seen {
		if !expiry.After(now) {
			delete(service.seen, key)
		}
	}
	key := domain + "/" + nonce
	if _, found := service.seen[key]; found {
		return ErrRemoteVRFDuplicateNonce
	}
	service.seen[key] = expires
	return nil
}

func (service *RemoteVRFService) audit(request *http.Request, authorized bool, reason string, nonce string) {
	if service.cfg.AuditSink == nil {
		return
	}
	service.cfg.AuditSink(RemoteVRFAuditEvent{
		Path:       request.URL.Path,
		RemoteAddr: request.RemoteAddr,
		Authorized: authorized,
		Reason:     reason,
		Nonce:      nonce,
		At:         service.now().UTC(),
	})
}

func decodeRemoteVRFJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeRemoteVRFJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeRemoteVRFError(writer http.ResponseWriter, status int, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "remote vrf error"
	}
	writeRemoteVRFJSON(writer, status, map[string]string{"error": message})
}
