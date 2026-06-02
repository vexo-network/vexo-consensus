package crypto

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestRemoteSignerRejectsInvalidConfig(t *testing.T) {
	if _, err := NewRemoteSigner("", []byte("public"), Ed25519Signer{}, 0); err != ErrMissingRemoteSignerURL {
		t.Fatalf("expected missing url, got %v", err)
	}
	if _, err := NewRemoteSigner("http://127.0.0.1", nil, Ed25519Signer{}, 0); err != ErrMissingRemotePublicKey {
		t.Fatalf("expected missing public key, got %v", err)
	}
}
