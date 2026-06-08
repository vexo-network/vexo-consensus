package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRemoteSignerSignsThroughHTTPAdapter(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		var payload remoteSignRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		message, err := base64.StdEncoding.DecodeString(payload.Message)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := baseSigner.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
	}))
	defer server.Close()

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("remote-sign")
	signature, err := remoteSigner.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !remoteSigner.Verify(baseSigner.PublicKey(), message, signature) {
		t.Fatal("expected remote signature to verify")
	}
}

func TestRemoteSignerUsesCallerContext(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := remoteSigner.SignWithContext(ctx, []byte("remote-sign")); err == nil {
		t.Fatal("expected canceled context to abort remote signer request")
	}
}

func TestRemoteSignerServiceRequiresAuthToken(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewRemoteSignerService(baseSigner, RemoteSignerPolicy{AuthToken: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()

	unauthenticated, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unauthenticated.Sign([]byte("message")); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected unauthenticated remote signer rejection, got %v", err)
	}
	authenticated, err := NewRemoteSignerWithAuth(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0, nil, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticated.Sign([]byte("message")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteSignerServicePersistsReplayNonces(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	noncePath := filepath.Join(t.TempDir(), "nonces.json")
	nonceGuard, err := NewFileBackedRemoteSignerNonceGuard(noncePath)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewRemoteSignerServiceWithNonceGuard(baseSigner, RemoteSignerPolicy{AuthToken: "secret"}, nil, nonceGuard)
	if err != nil {
		t.Fatal(err)
	}
	payload := remoteSignRequest{
		PublicKey: base64.StdEncoding.EncodeToString(baseSigner.PublicKey()),
		Message:   base64.StdEncoding.EncodeToString([]byte("message")),
		Nonce:     "nonce-1",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	request, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	server.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected first nonce to be accepted, got status %d", response.StatusCode)
	}
	restoredNonceGuard, err := NewFileBackedRemoteSignerNonceGuard(noncePath)
	if err != nil {
		t.Fatal(err)
	}
	restoredService, err := NewRemoteSignerServiceWithNonceGuard(baseSigner, RemoteSignerPolicy{AuthToken: "secret"}, nil, restoredNonceGuard)
	if err != nil {
		t.Fatal(err)
	}
	restoredServer := httptest.NewServer(restoredService)
	defer restoredServer.Close()
	replayRequest, err := http.NewRequest(http.MethodPost, restoredServer.URL, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	replayRequest.Header.Set("Authorization", "Bearer secret")
	replayResponse, err := http.DefaultClient.Do(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer replayResponse.Body.Close()
	if replayResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected restored nonce guard to reject replay, got status %d", replayResponse.StatusCode)
	}
}

func TestRemoteSignerSendsSignPolicy(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	var observed remoteSignRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&observed); err != nil {
			t.Fatal(err)
		}
		message, err := base64.StdEncoding.DecodeString(observed.Message)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := baseSigner.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
	}))
	defer server.Close()

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	policy := SignPolicy{ChainID: "vexo-test", Height: 7, Round: 1, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	if _, err := remoteSigner.SignWithPolicy(policy, []byte("vote")); err != nil {
		t.Fatal(err)
	}
	if observed.Policy == nil || observed.Policy.Height != 7 || observed.Policy.Type != SignTypeConsensusVote || observed.Policy.Domain != DomainConsensusVote {
		t.Fatalf("expected sign policy to be sent, got %+v", observed.Policy)
	}
}

func TestDoubleSignGuardRejectsConflictingMessage(t *testing.T) {
	guard := NewDoubleSignGuard()
	policy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 0, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	if err := guard.CheckAndRemember(policy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckAndRemember(policy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckAndRemember(policy, []byte("block-b")); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected double sign rejection, got %v", err)
	}
}

func TestDoubleSignGuardSeparatesDomainsAndRestoresSnapshot(t *testing.T) {
	guard := NewDoubleSignGuard()
	votePolicy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 0, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	timeoutPolicy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 0, Type: SignTypeConsensusTimeoutVote, Domain: DomainConsensusTimeoutVote}
	if err := guard.CheckAndRemember(votePolicy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckAndRemember(timeoutPolicy, []byte("timeout-a")); err != nil {
		t.Fatal(err)
	}

	restored, err := NewDoubleSignGuardFromSnapshot(guard.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.CheckAndRemember(votePolicy, []byte("block-b")); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected restored guard double-sign rejection, got %v", err)
	}
}

func TestRemoteSignerGuardRejectsBeforeHTTP(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		var payload remoteSignRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		message, err := base64.StdEncoding.DecodeString(payload.Message)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := baseSigner.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
	}))
	defer server.Close()

	remoteSigner, err := NewRemoteSignerWithGuard(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0, NewDoubleSignGuard())
	if err != nil {
		t.Fatal(err)
	}
	policy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 0, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	if _, err := remoteSigner.SignWithPolicy(policy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.SignWithPolicy(policy, []byte("block-b")); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected local double-sign guard rejection, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected only first request to reach remote signer, got %d", requests)
	}
}

func TestRemoteSignerServiceEnforcesPolicyAndDoubleSignGuard(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewRemoteSignerService(baseSigner, RemoteSignerPolicy{
		ChainID:       "vexo-test",
		AllowedTypes:  []SignType{SignTypeConsensusVote},
		RequirePolicy: true,
	}, NewDoubleSignGuard())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	policy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 0, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	if _, err := remoteSigner.SignWithPolicy(policy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.SignWithPolicy(policy, []byte("block-b")); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected remote double-sign rejection, got %v", err)
	}
	wrongChain := policy
	wrongChain.ChainID = "other"
	if _, err := remoteSigner.SignWithPolicy(wrongChain, []byte("block-c")); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected remote policy rejection, got %v", err)
	}
	if _, err := remoteSigner.Sign([]byte("no-policy")); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected missing policy rejection, got %v", err)
	}
}

func TestFileBackedDoubleSignGuardPersistsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	guard, err := NewFileBackedDoubleSignGuard(path)
	if err != nil {
		t.Fatal(err)
	}
	policy := SignPolicy{ChainID: "vexo-test", Height: 2, Round: 1, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
	if err := guard.CheckAndRemember(policy, []byte("block-a")); err != nil {
		t.Fatal(err)
	}
	restored, err := NewFileBackedDoubleSignGuard(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.CheckAndRemember(policy, []byte("block-b")); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected persisted double-sign rejection, got %v", err)
	}
	if err := restored.CheckAndRemember(policy, []byte("block-a")); err != nil {
		t.Fatalf("expected same payload to be allowed, got %v", err)
	}
}

func TestSignPolicyRejectsTypeDomainMismatch(t *testing.T) {
	policy := SignPolicy{ChainID: "vexo-test", Height: 1, Type: SignTypeConsensusVote, Domain: DomainConsensusProposal}
	if err := policy.Validate(); !errors.Is(err, ErrInvalidSignPolicy) {
		t.Fatalf("expected invalid sign policy, got %v", err)
	}
}

func TestRemoteSignerRejectsInvalidConfig(t *testing.T) {
	if _, err := NewRemoteSigner("", []byte("public"), Ed25519Signer{}, 0); err != ErrMissingRemoteSignerURL {
		t.Fatalf("expected missing url, got %v", err)
	}
	if _, err := NewRemoteSigner("http://127.0.0.1", nil, Ed25519Signer{}, 0); err != ErrMissingRemotePublicKey {
		t.Fatalf("expected missing public key, got %v", err)
	}
}
