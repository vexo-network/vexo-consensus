package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vexo-network/vexo-consensus/signbytes"
	"github.com/vexo-network/vexo-consensus/types"
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

func TestRemoteSignerRejectsInvalidRemoteSignature(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString([]byte("not-a-valid-signature"))})
	}))
	defer server.Close()

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.Sign([]byte("remote-sign")); !errors.Is(err, ErrRemoteSignatureInvalid) {
		t.Fatalf("expected invalid remote signature rejection, got %v", err)
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

func TestRemoteSignerBearerAuthUsesExactBearerToken(t *testing.T) {
	if !remoteSignerBearerAuthorized("Bearer secret", "secret") {
		t.Fatal("expected exact bearer token to authorize")
	}
	for _, header := range []string{"secret", "Bearer secret ", "Bearer SECRET", "Bearer secret\n", "Bearer wrong"} {
		if remoteSignerBearerAuthorized(header, "secret") {
			t.Fatalf("expected header %q to be rejected", header)
		}
	}
}

func TestRemoteSignerNonceIsRandomHex(t *testing.T) {
	first, err := newRemoteSignerNonce()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newRemoteSignerNonce()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("expected 16-byte hex nonces, got %q %q", first, second)
	}
	if first == second {
		t.Fatal("expected unique nonces")
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

// statementPayload lays out a consensus statement the way consensus.ProposalSignBytes
// and consensus.TimeoutVoteSignBytes do: tag, height, view, then the fixed-width rest.
func statementPayload(tag string, height types.Height, round types.Round, rest ...[]byte) []byte {
	payload := append([]byte(tag), binary.BigEndian.AppendUint64(nil, uint64(height))...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(round))
	for _, part := range rest {
		payload = append(payload, part...)
	}
	return payload
}

func filled(size int, value byte) []byte {
	return bytes.Repeat([]byte{value}, size)
}

func wrap(t *testing.T, domain Domain, payload []byte) []byte {
	t.Helper()
	message, err := DomainMessage(domain, payload)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func voteMessage(t *testing.T, height types.Height, round types.Round, block byte) []byte {
	t.Helper()
	return wrap(t, DomainConsensusVote, signbytes.Vote(height, round, types.Hash(filled(32, block))))
}

func timeoutVoteMessage(t *testing.T, height types.Height, round types.Round, highQC byte) []byte {
	t.Helper()
	return wrap(t, DomainConsensusTimeoutVote, statementPayload("timeout_vote", height, round, filled(signQuorumCertSize, highQC)))
}

func proposalMessage(t *testing.T, height types.Height, round types.Round, proposer string, block byte) []byte {
	t.Helper()
	return wrap(t, DomainConsensusProposal, statementPayload("proposal", height, round,
		[]byte(proposer), filled(signHashSize, block), filled(signQuorumCertSize, 1), filled(signTimeoutCertSize, 2)))
}

func votePolicy(height types.Height, round types.Round) SignPolicy {
	return SignPolicy{ChainID: "vexo-test", Height: height, Round: round, Type: SignTypeConsensusVote, Domain: DomainConsensusVote}
}

func signingServer(t *testing.T, signer Signer, handler func(remoteSignRequest)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload remoteSignRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if handler != nil {
			handler(payload)
		}
		message, err := base64.StdEncoding.DecodeString(payload.Message)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := signer.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(remoteSignResponse{Signature: base64.StdEncoding.EncodeToString(signature)})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRemoteSignerSendsSignPolicy(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	var observed remoteSignRequest
	server := signingServer(t, baseSigner, func(payload remoteSignRequest) { observed = payload })

	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	policy := votePolicy(7, 1)
	if _, err := remoteSigner.SignWithPolicy(policy, voteMessage(t, 7, 1, 'a')); err != nil {
		t.Fatal(err)
	}
	if observed.Policy == nil || observed.Policy.Height != 7 || observed.Policy.Round != 1 || observed.Policy.Type != SignTypeConsensusVote || observed.Policy.Domain != DomainConsensusVote {
		t.Fatalf("expected sign policy to be sent, got %+v", observed.Policy)
	}
}

func TestRemoteSignerRefusesPolicyThatDoesNotDescribeTheBytes(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := signingServer(t, baseSigner, func(remoteSignRequest) { requests++ })
	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	vote := voteMessage(t, 7, 3, 'a')
	timeoutPolicy := SignPolicy{ChainID: "vexo-test", Height: 7, Round: 3, Type: SignTypeConsensusTimeoutVote, Domain: DomainConsensusTimeoutVote}
	for name, policy := range map[string]SignPolicy{
		"other round":  votePolicy(7, 4),
		"other height": votePolicy(8, 3),
		"other type":   timeoutPolicy,
	} {
		if _, err := remoteSigner.SignWithPolicy(policy, vote); !errors.Is(err, ErrInvalidSignPolicy) {
			t.Fatalf("%s: expected invalid sign policy, got %v", name, err)
		}
	}
	if _, err := remoteSigner.SignWithPolicy(votePolicy(7, 3), []byte("not a statement")); !errors.Is(err, ErrInvalidSignPolicy) {
		t.Fatalf("expected raw bytes to be refused under a consensus policy, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no request to reach the signer, got %d", requests)
	}
}

func TestDoubleSignGuardHighWaterMark(t *testing.T) {
	guard := NewDoubleSignGuard()
	key := types.PublicKey("validator-key")
	if err := guard.CheckAndRemember(key, voteMessage(t, 5, 10, 'a')); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckAndRemember(key, voteMessage(t, 5, 10, 'a')); err != nil {
		t.Fatalf("expected the identical statement at the mark to be signable again, got %v", err)
	}
	if err := guard.CheckAndRemember(key, voteMessage(t, 5, 10, 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected conflicting statement at the mark to be refused, got %v", err)
	}
	// The same view is one slot even when the height differs: a vote for another block at
	// another height in the same view is still a second vote in that view.
	if err := guard.CheckAndRemember(key, voteMessage(t, 6, 10, 'a')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected a vote at another height in the same view to be refused, got %v", err)
	}
	for _, round := range []types.Round{9, 3, 0} {
		if err := guard.CheckAndRemember(key, voteMessage(t, 5, round, 'a')); !errors.Is(err, ErrDoubleSign) {
			t.Fatalf("expected view %d below the mark to be refused, got %v", round, err)
		}
	}
	// A view above the mark advances it even to a lower height: after a fork the next
	// view legitimately extends an older block.
	if err := guard.CheckAndRemember(key, voteMessage(t, 4, 11, 'c')); err != nil {
		t.Fatalf("expected a higher view to advance the mark, got %v", err)
	}
	if err := guard.CheckAndRemember(key, voteMessage(t, 5, 10, 'a')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected the old statement to be refused once the mark moved past it, got %v", err)
	}
}

func TestDoubleSignGuardKeepsMarksPerKeyAndKind(t *testing.T) {
	guard := NewDoubleSignGuard()
	first, second := types.PublicKey("first"), types.PublicKey("second")
	if err := guard.CheckAndRemember(first, voteMessage(t, 1, 8, 'a')); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckAndRemember(first, timeoutVoteMessage(t, 1, 2, 'a')); err != nil {
		t.Fatalf("expected the timeout vote mark to be independent of the vote mark, got %v", err)
	}
	if err := guard.CheckAndRemember(first, proposalMessage(t, 1, 3, "validator-1", 'a')); err != nil {
		t.Fatalf("expected the proposal mark to be independent of the vote mark, got %v", err)
	}
	if err := guard.CheckAndRemember(second, voteMessage(t, 1, 1, 'z')); err != nil {
		t.Fatalf("expected another key to have its own mark, got %v", err)
	}
	if err := guard.CheckAndRemember(first, timeoutVoteMessage(t, 1, 2, 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected a conflicting timeout vote to be refused, got %v", err)
	}
	if err := guard.CheckAndRemember(first, proposalMessage(t, 1, 3, "validator-1", 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected a conflicting proposal to be refused, got %v", err)
	}
}

func TestDoubleSignGuardIgnoresWhatPolicyDeclares(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewRemoteSignerService(baseSigner, RemoteSignerPolicy{}, NewDoubleSignGuard())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()
	post := func(message []byte, policy *SignPolicy) int {
		t.Helper()
		encoded, err := json.Marshal(remoteSignRequest{
			PublicKey: base64.StdEncoding.EncodeToString(baseSigner.PublicKey()),
			Message:   base64.StdEncoding.EncodeToString(message),
			Policy:    policy,
		})
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.Post(server.URL, "application/json", bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		return response.StatusCode
	}

	declared := votePolicy(5, 10)
	if status := post(voteMessage(t, 5, 10, 'a'), &declared); status != http.StatusOK {
		t.Fatalf("expected first vote to be signed, got status %d", status)
	}
	conflicting := voteMessage(t, 5, 10, 'b')
	laterView := votePolicy(5, 11)
	otherChain := declared
	otherChain.ChainID = "other-chain"
	otherType := SignPolicy{ChainID: "vexo-test", Height: 5, Round: 10, Type: SignTypeConsensusTimeoutVote, Domain: DomainConsensusTimeoutVote}
	for name, policy := range map[string]*SignPolicy{
		"same policy":             &declared,
		"policy for a later view": &laterView,
		"policy of another chain": &otherChain,
		"policy of another type":  &otherType,
		"no policy at all":        nil,
	} {
		if status := post(conflicting, policy); status == http.StatusOK {
			t.Fatalf("%s: conflicting vote was signed", name)
		}
	}
	// The same bytes re-wrapped in another consensus domain are not a vote any more.
	rewrapped := wrap(t, DomainConsensusProposal, signbytes.Vote(5, 10, types.Hash(filled(32, 'b'))))
	if status := post(rewrapped, nil); status == http.StatusOK {
		t.Fatal("vote payload under the proposal domain was signed")
	}
	// A vote at another view is still signable, so the refusals above are not blanket.
	if status := post(voteMessage(t, 5, 11, 'b'), &laterView); status != http.StatusOK {
		t.Fatalf("expected a vote in a later view to be signed, got status %d", status)
	}
}

func TestRemoteSignerGuardRejectsBeforeHTTP(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := signingServer(t, baseSigner, func(remoteSignRequest) { requests++ })

	remoteSigner, err := NewRemoteSignerWithGuard(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0, NewDoubleSignGuard())
	if err != nil {
		t.Fatal(err)
	}
	policy := votePolicy(1, 0)
	if _, err := remoteSigner.SignWithPolicy(policy, voteMessage(t, 1, 0, 'a')); err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.SignWithPolicy(policy, voteMessage(t, 1, 0, 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected local double-sign guard rejection, got %v", err)
	}
	// Dropping the policy does not drop the guard.
	if _, err := remoteSigner.Sign(voteMessage(t, 1, 0, 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected the guard to apply to unpoliced signing, got %v", err)
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
	policy := votePolicy(1, 0)
	if _, err := remoteSigner.SignWithPolicy(policy, voteMessage(t, 1, 0, 'a')); err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.SignWithPolicy(policy, voteMessage(t, 1, 0, 'b')); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected remote double-sign rejection, got %v", err)
	}
	wrongChain := votePolicy(1, 1)
	wrongChain.ChainID = "other"
	if _, err := remoteSigner.SignWithPolicy(wrongChain, voteMessage(t, 1, 1, 'c')); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected remote policy rejection, got %v", err)
	}
	timeoutPolicy := SignPolicy{ChainID: "vexo-test", Height: 1, Round: 1, Type: SignTypeConsensusTimeoutVote, Domain: DomainConsensusTimeoutVote}
	if _, err := remoteSigner.SignWithPolicy(timeoutPolicy, timeoutVoteMessage(t, 1, 1, 'a')); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected disallowed sign type rejection, got %v", err)
	}
	if _, err := remoteSigner.Sign([]byte("no-policy")); !errors.Is(err, ErrRemoteSignerRejected) {
		t.Fatalf("expected missing policy rejection, got %v", err)
	}
}

func TestRemoteSignerServiceSignsProbeWithoutMovingMarks(t *testing.T) {
	baseSigner, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	guard := NewDoubleSignGuard()
	service, err := NewRemoteSignerService(baseSigner, RemoteSignerPolicy{ChainID: "vexo-test", RequirePolicy: true}, guard)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()
	remoteSigner, err := NewRemoteSigner(server.URL, baseSigner.PublicKey(), Ed25519Signer{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	probePolicy := SignPolicy{ChainID: "vexo-test", Height: 1, Type: SignTypeRemoteSignerProbe, Domain: DomainRemoteSignerProbe}
	for _, challenge := range []string{"kms-check", "kms-check", "another challenge"} {
		message := wrap(t, DomainRemoteSignerProbe, []byte(challenge))
		signature, err := remoteSigner.SignWithPolicy(probePolicy, message)
		if err != nil {
			t.Fatal(err)
		}
		if !remoteSigner.Verify(baseSigner.PublicKey(), message, signature) {
			t.Fatal("expected probe signature to verify")
		}
	}
	if len(guard.marks) != 0 {
		t.Fatalf("expected probes to leave the guard untouched, got %+v", guard.marks)
	}
	// A probe signature cannot stand in for a consensus statement.
	vote := voteMessage(t, 1, 0, 'a')
	if _, err := remoteSigner.SignWithPolicy(probePolicy, vote); !errors.Is(err, ErrInvalidSignPolicy) {
		t.Fatalf("expected a vote to be refused under the probe policy, got %v", err)
	}
}

func TestFileBackedDoubleSignGuardPersistsHighWaterMark(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	guard, err := NewFileBackedDoubleSignGuard(path)
	if err != nil {
		t.Fatal(err)
	}
	key := types.PublicKey("validator-key")
	if err := guard.CheckAndRemember(key, voteMessage(t, 2, 1, 'a')); err != nil {
		t.Fatal(err)
	}
	restored, err := NewFileBackedDoubleSignGuard(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.CheckAndRemember(key, voteMessage(t, 2, 1, 'b')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected persisted double-sign rejection, got %v", err)
	}
	if err := restored.CheckAndRemember(key, voteMessage(t, 2, 0, 'a')); !errors.Is(err, ErrDoubleSign) {
		t.Fatalf("expected persisted mark to refuse an older view, got %v", err)
	}
	if err := restored.CheckAndRemember(key, voteMessage(t, 2, 1, 'a')); err != nil {
		t.Fatalf("expected same payload to be allowed, got %v", err)
	}
	if err := restored.CheckAndRemember(key, voteMessage(t, 2, 2, 'b')); err != nil {
		t.Fatalf("expected a later view to advance the persisted mark, got %v", err)
	}
}

func TestFileBackedDoubleSignGuardStateStaysBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	guard, err := NewFileBackedDoubleSignGuard(path)
	if err != nil {
		t.Fatal(err)
	}
	key := types.PublicKey("validator-key")
	signViews := func(from, to types.Round) int64 {
		t.Helper()
		for round := from; round < to; round++ {
			if err := guard.CheckAndRemember(key, voteMessage(t, types.Height(round)+1, round, 'a')); err != nil {
				t.Fatal(err)
			}
			if err := guard.CheckAndRemember(key, timeoutVoteMessage(t, types.Height(round)+1, round, 'a')); err != nil {
				t.Fatal(err)
			}
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Size()
	}
	small := signViews(1, 4)
	large := signViews(4, 400)
	if large > small+64 {
		t.Fatalf("expected persisted state to stay bounded, grew from %d to %d bytes", small, large)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the guard file to remain, got %d entries", len(entries))
	}
}

func TestDoubleSignGuardDoesNotAdvanceWhenPersistenceFails(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	guard := NewDoubleSignGuard()
	guard.path = filepath.Join(blocker, "guard.json")
	key := types.PublicKey("validator-key")
	vote := voteMessage(t, 1, 1, 'a')
	if err := guard.CheckAndRemember(key, vote); err == nil {
		t.Fatal("expected a failed durable write to refuse the signature")
	}
	// The mark must not have moved: an unrecorded signature would be unprotected after a restart.
	if err := guard.CheckAndRemember(key, vote); err == nil {
		t.Fatal("expected the retry to fail again instead of being treated as already recorded")
	}
	if len(guard.marks) != 0 {
		t.Fatalf("expected no in-memory mark after failed persistence, got %+v", guard.marks)
	}
}

func TestDoubleSignGuardRefusesStateFromOlderFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	legacy := `{"schema_version":"v1","records":{"vexo-test/1/0/consensus_vote/vexo.consensus.vote.v1":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileBackedDoubleSignGuard(path); !errors.Is(err, ErrUnsupportedKeyVersion) {
		t.Fatalf("expected older guard state to be refused, got %v", err)
	}
}

func TestDoubleSignGuardRefusesCorruptState(t *testing.T) {
	for name, document := range map[string]string{
		"unknown kind":   `{"schema_version":"high-water-mark/v1","high_water_marks":{"a2V5":{"remote_signer_probe":{"view":1,"digest":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}}}`,
		"short digest":   `{"schema_version":"high-water-mark/v1","high_water_marks":{"a2V5":{"consensus_vote":{"view":1,"digest":"AAAA"}}}}`,
		"bad public key": `{"schema_version":"high-water-mark/v1","high_water_marks":{"***":{}}}`,
		"extra field":    `{"schema_version":"high-water-mark/v1","high_water_marks":{},"records":{}}`,
	} {
		path := filepath.Join(t.TempDir(), "guard.json")
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewFileBackedDoubleSignGuard(path); err == nil {
			t.Fatalf("%s: expected corrupt guard state to be refused", name)
		}
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
