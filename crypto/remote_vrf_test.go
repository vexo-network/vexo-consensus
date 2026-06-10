package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestRemoteVRFAdapterProvesAndVerifiesThroughHTTP(t *testing.T) {
	publicKey := types.PublicKey("public")
	seed := []byte("seed")
	output := []byte("output")
	proof := []byte("proof")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/prove":
			var payload remoteVRFProveRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode prove request: %v", err)
			}
			if payload.PublicKey != base64.StdEncoding.EncodeToString(publicKey) || payload.Seed != base64.StdEncoding.EncodeToString(seed) {
				t.Fatalf("unexpected prove request: %+v", payload)
			}
			if payload.Nonce == "" || payload.Domain != "vexo.remote_vrf.prove.v1" || payload.IssuedAtUnixNano == 0 || payload.DeadlineUnixNano <= payload.IssuedAtUnixNano {
				t.Fatalf("expected replay-bound prove challenge, got %+v", payload)
			}
			_ = json.NewEncoder(writer).Encode(remoteVRFProveResponse{
				Output: base64.StdEncoding.EncodeToString(output),
				Proof:  base64.StdEncoding.EncodeToString(proof),
				Nonce:  payload.Nonce,
			})
		case "/verify":
			var payload remoteVRFVerifyRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode verify request: %v", err)
			}
			if payload.Nonce == "" || payload.Domain != "vexo.remote_vrf.verify.v1" || payload.IssuedAtUnixNano == 0 || payload.DeadlineUnixNano <= payload.IssuedAtUnixNano {
				t.Fatalf("expected replay-bound verify challenge, got %+v", payload)
			}
			valid := payload.Output == base64.StdEncoding.EncodeToString(output) &&
				payload.Proof == base64.StdEncoding.EncodeToString(proof)
			_ = json.NewEncoder(writer).Encode(remoteVRFVerifyResponse{Valid: valid, Nonce: payload.Nonce})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	adapter, err := NewRemoteVRFAdapter(config.VRFConfig{
		AdapterName:         VRFAdapterRemoteHTTPName,
		ProductionAdapter:   true,
		AuditReport:         "remote-vrf-audit",
		DependencyAudit:     "external:remote-vrf-service-audit-2026",
		AuditEvidenceSHA256: strings.Repeat("b", 64),
		KeySource:           "remote-http:" + server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	actualOutput, actualProof, err := adapter.Prove(publicKey, seed)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualOutput) != string(output) || string(actualProof) != string(proof) {
		t.Fatalf("unexpected proof output=%q proof=%q", actualOutput, actualProof)
	}
	if !adapter.Verify(publicKey, seed, actualOutput, actualProof) {
		t.Fatalf("expected remote proof to verify")
	}
	if err := ValidateVRFAdapter(adapter, config.VRFConfig{
		AdapterName:         VRFAdapterRemoteHTTPName,
		ProductionAdapter:   true,
		AuditReport:         "remote-vrf-audit",
		DependencyAudit:     "external:remote-vrf-service-audit-2026",
		AuditEvidenceSHA256: strings.Repeat("b", 64),
		KeySource:           "remote-http:" + server.URL,
	}); err != nil {
		t.Fatalf("expected remote adapter metadata to validate: %v", err)
	}
}

func TestRemoteVRFAdapterHonorsCanceledContext(t *testing.T) {
	adapter, err := NewRemoteVRFAdapter(config.VRFConfig{
		AdapterName:         VRFAdapterRemoteHTTPName,
		ProductionAdapter:   true,
		AuditReport:         "remote-vrf-audit",
		DependencyAudit:     "external:remote-vrf-service-audit-2026",
		AuditEvidenceSHA256: strings.Repeat("b", 64),
		KeySource:           "remote-http:http://127.0.0.1:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	remoteAdapter, ok := adapter.(RemoteVRFAdapter)
	if !ok {
		t.Fatalf("expected remote VRF adapter, got %T", adapter)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := remoteAdapter.ProveWithContext(ctx, types.PublicKey("public"), []byte("seed")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled prove context, got %v", err)
	}
	if remoteAdapter.VerifyWithContext(ctx, types.PublicKey("public"), []byte("seed"), []byte("output"), []byte("proof")) {
		t.Fatalf("expected canceled verify context to fail closed")
	}
}

func TestRemoteVRFAdapterRejectsPartialTLSConfig(t *testing.T) {
	_, err := NewRemoteVRFAdapter(config.VRFConfig{
		AdapterName:       VRFAdapterRemoteHTTPName,
		ProductionAdapter: true,
		AuditReport:       "remote-vrf-audit",
		KeySource:         "remote-http:https://vrf.example",
		TLSCertPath:       "client.crt",
	})
	if !errors.Is(err, ErrInvalidRemoteVRFTLS) {
		t.Fatalf("expected invalid TLS config, got %v", err)
	}
}

func TestRemoteVRFServiceProvesVerifiesAndRejectsReplay(t *testing.T) {
	privateKey, err := generateECVRFP256PrivateKeyBytes()
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	localVRF, err := NewECVRFP256Adapter(config.VRFConfig{
		Keys: map[string][]byte{base64.StdEncoding.EncodeToString(publicKey): privateKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := make([]RemoteVRFAuditEvent, 0)
	service, err := NewRemoteVRFService(localVRF, RemoteVRFServiceConfig{
		AuthToken: "secret",
		AuditSink: func(event RemoteVRFAuditEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()

	remoteVRF, err := NewRemoteVRFAdapterWithAuth(config.VRFConfig{
		AdapterName:     VRFAdapterRemoteHTTPName,
		AuditReport:     "remote-vrf-audit",
		DependencyAudit: "external:remote-vrf-service-audit",
		KeySource:       "remote-http:" + server.URL,
	}, "secret")
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := remoteVRF.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !remoteVRF.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected remote VRF proof to verify")
	}

	challenge, err := newRemoteVRFChallenge()
	if err != nil {
		t.Fatal(err)
	}
	requestBody, err := json.Marshal(remoteVRFVerifyRequest{
		PublicKey:        base64.StdEncoding.EncodeToString(publicKey),
		Seed:             base64.StdEncoding.EncodeToString([]byte("seed")),
		Output:           base64.StdEncoding.EncodeToString(output),
		Proof:            base64.StdEncoding.EncodeToString(proof),
		Nonce:            challenge.nonce,
		IssuedAtUnixNano: challenge.issuedAt,
		DeadlineUnixNano: challenge.deadline,
		Domain:           remoteVRFVerifyDomain,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/verify", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer secret")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if index == 0 && response.StatusCode != http.StatusOK {
			t.Fatalf("expected first nonce use to pass, got status %d", response.StatusCode)
		}
		if index == 1 && response.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected replay to fail, got status %d", response.StatusCode)
		}
	}
	if len(events) == 0 {
		t.Fatal("expected remote VRF audit events")
	}
}

func TestRemoteVRFServiceRequiresAuthToken(t *testing.T) {
	localVRF, err := NewECVRFP256Adapter(config.VRFConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewRemoteVRFService(localVRF, RemoteVRFServiceConfig{AuthToken: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/verify", strings.NewReader(`{}`))
	service.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", response.Code)
	}
}
