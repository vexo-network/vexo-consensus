package crypto

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			_ = json.NewEncoder(writer).Encode(remoteVRFProveResponse{
				Output: base64.StdEncoding.EncodeToString(output),
				Proof:  base64.StdEncoding.EncodeToString(proof),
			})
		case "/verify":
			var payload remoteVRFVerifyRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode verify request: %v", err)
			}
			valid := payload.Output == base64.StdEncoding.EncodeToString(output) &&
				payload.Proof == base64.StdEncoding.EncodeToString(proof)
			_ = json.NewEncoder(writer).Encode(remoteVRFVerifyResponse{Valid: valid})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	adapter, err := NewRemoteVRFAdapter(config.VRFConfig{
		AdapterName:       VRFAdapterRemoteHTTPName,
		ProductionAdapter: true,
		AuditReport:       "remote-vrf-audit",
		KeySource:         "remote-http:" + server.URL,
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
		AdapterName:       VRFAdapterRemoteHTTPName,
		ProductionAdapter: true,
		AuditReport:       "remote-vrf-audit",
		KeySource:         "remote-http:" + server.URL,
	}); err != nil {
		t.Fatalf("expected remote adapter metadata to validate: %v", err)
	}
}
