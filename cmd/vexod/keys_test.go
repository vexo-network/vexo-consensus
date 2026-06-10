package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

func TestRunKeysGenAndShow(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	if !strings.Contains(output, "generated validator key") || !strings.Contains(output, "public_key:") {
		t.Fatalf("unexpected keys gen output:\n%s", output)
	}
	if strings.Contains(output, "private_key") {
		t.Fatalf("key generation output leaked private key:\n%s", output)
	}
	keyPath := filepath.Join(home, keyFileName)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if signer, err := document.SignerWithPassphrase(""); err != nil || signer == nil {
		t.Fatal(err)
	}

	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home}); err != nil {
		t.Fatal(err)
	}
	showOutput := buffer.String()
	if !strings.Contains(showOutput, "validator key") || !strings.Contains(showOutput, document.PublicKey) {
		t.Fatalf("unexpected keys show output:\n%s", showOutput)
	}
	if strings.Contains(showOutput, document.PrivateKey) || strings.Contains(showOutput, "private_key") {
		t.Fatalf("key show output leaked private key:\n%s", showOutput)
	}
}

func TestRunKeysGenBLS(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home, "--type", "bls"}); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	if !strings.Contains(output, "type: bls") || strings.Contains(output, "private_key") {
		t.Fatalf("unexpected BLS key output:\n%s", output)
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.Type != vexocrypto.KeyTypeBLS || document.Metadata.BLSProofOfPossession == "" || document.Metadata.BLSAdapter == "" {
		t.Fatalf("unexpected BLS key document: %+v", document)
	}
	if document.Metadata.BLSAdapter != vexocrypto.BLSAdapterBLSTName {
		t.Fatalf("expected BLST adapter metadata, got %+v", document.Metadata)
	}
	signer, err := document.SignerWithPassphrase("")
	if err != nil {
		t.Fatal(err)
	}
	blsSigner, ok := signer.(vexocrypto.BLSAdapter)
	if !ok {
		t.Fatalf("expected BLS adapter signer, got %T", signer)
	}
	proof, err := base64.StdEncoding.DecodeString(document.Metadata.BLSProofOfPossession)
	if err != nil {
		t.Fatal(err)
	}
	if !blsSigner.VerifyProofOfPossession(blsSigner.PublicKey(), proof) {
		t.Fatal("expected BLS proof of possession to verify")
	}
}

func TestRunKeysGenBLSAllowsCIRCLReferenceAdapter(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--type", "bls", "--bls-adapter", vexocrypto.BLSAdapterCIRCLName}); err != nil {
		t.Fatal(err)
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.Metadata.BLSAdapter != vexocrypto.BLSAdapterCIRCLName {
		t.Fatalf("expected CIRCL reference adapter metadata, got %+v", document.Metadata)
	}
}

func TestRunKeysGenVRF(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home, "--type", "vrf"}); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	if !strings.Contains(output, "type: vrf") || strings.Contains(output, "private_key") {
		t.Fatalf("unexpected VRF key output:\n%s", output)
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.Type != vexocrypto.KeyTypeVRF || document.Metadata.VRFAdapter != vexocrypto.VRFAdapterECVRFP256Name {
		t.Fatalf("unexpected VRF key document: %+v", document)
	}
	privateKey, err := document.ECVRFP256PrivateKeyWithPassphrase("")
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := vexocrypto.ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if document.PublicKey != base64.StdEncoding.EncodeToString(publicKey) {
		t.Fatalf("unexpected VRF public key: %s", document.PublicKey)
	}
}

func TestRunKeysVerifyVRF(t *testing.T) {
	privateKey, err := vexocrypto.GenerateECVRFP256KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	privateKeyBytes, err := privateKey.ECVRFP256PrivateKeyWithPassphrase("")
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := vexocrypto.ECVRFP256PublicKeyFromPrivateKey(privateKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	localVRF, err := vexocrypto.NewECVRFP256Adapter(config.VRFConfig{
		Keys: map[string][]byte{base64.StdEncoding.EncodeToString(publicKey): privateKeyBytes},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := vexocrypto.NewRemoteVRFService(localVRF, vexocrypto.RemoteVRFServiceConfig{AuthToken: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()

	var output bytes.Buffer
	if err := runKeys(&output, []string{"verify-vrf", "--url", server.URL, "--public-key", base64.StdEncoding.EncodeToString(publicKey), "--seed", "check", "--auth-token", "secret"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "remote vrf verified") || !strings.Contains(output.String(), "proof:") {
		t.Fatalf("unexpected verify-vrf output:\n%s", output.String())
	}
}

func TestRunKeysShowJSON(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"show", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.SchemaVersion != vexocrypto.KeyDocumentVersionV1 || info.Type != vexocrypto.KeyTypeBLS || info.PublicKey == "" {
		t.Fatalf("unexpected key info: %+v", info)
	}
	if strings.Contains(buffer.String(), "private_key") {
		t.Fatalf("key json output leaked private key:\n%s", buffer.String())
	}
}

func TestRunKeysGenEncryptedAndShow(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home, "--encrypt", "--passphrase", "secret", "--id", "key-1", "--active-from", "10", "--active-until", "20"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "encrypted: true") {
		t.Fatalf("expected encrypted output, got:\n%s", buffer.String())
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.PrivateKey != "" || document.Encryption == nil {
		t.Fatalf("expected encrypted key on disk: %+v", document)
	}
	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home, "--passphrase", "secret", "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if !info.Encrypted || info.KeyID != "key-1" || info.ActiveFrom != 10 || info.ActiveUntil != 20 {
		t.Fatalf("unexpected encrypted key info: %+v", info)
	}
	if strings.Contains(buffer.String(), "private_key") {
		t.Fatalf("key json output leaked private key:\n%s", buffer.String())
	}
}

func TestRunKeysRemoteAndShow(t *testing.T) {
	home := t.TempDir()
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	encodedPublicKey := base64.StdEncoding.EncodeToString(signer.PublicKey())

	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"remote", "--home", home, "--public-key", encodedPublicKey, "--url", "http://127.0.0.1:9000/sign", "--id", "remote-1", "--active-from", "10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "registered remote validator key") {
		t.Fatalf("unexpected remote key output:\n%s", buffer.String())
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.Type != vexocrypto.KeyTypeRemote || document.Metadata.RemoteURL == "" || document.Metadata.ActiveFrom != 10 {
		t.Fatalf("unexpected remote key document: %+v", document)
	}

	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Type != vexocrypto.KeyTypeRemote || info.RemoteURL != "http://127.0.0.1:9000/sign" || info.KeyID != "remote-1" {
		t.Fatalf("unexpected remote key info: %+v", info)
	}
}

func TestRunKeysVerifyRemote(t *testing.T) {
	home := t.TempDir()
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	var observedPolicy *vexocrypto.SignPolicy
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Message string                 `json:"message"`
			Policy  *vexocrypto.SignPolicy `json:"policy"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		observedPolicy = payload.Policy
		message, err := base64.StdEncoding.DecodeString(payload.Message)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := signer.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(map[string]string{"signature": base64.StdEncoding.EncodeToString(signature)})
	}))
	defer server.Close()

	if err := runKeys(&bytes.Buffer{}, []string{"remote", "--home", home, "--public-key", base64.StdEncoding.EncodeToString(signer.PublicKey()), "--url", server.URL}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runKeys(&output, []string{"verify-remote", "--home", home, "--challenge", "kms-check"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "remote signer verified") || !strings.Contains(output.String(), "signature:") || !strings.Contains(output.String(), "policy: "+defaultChainID+"/1/0/consensus_vote/vexo.consensus.vote.v1") {
		t.Fatalf("unexpected verify remote output:\n%s", output.String())
	}
	if observedPolicy == nil || observedPolicy.ChainID != defaultChainID || observedPolicy.Height != 1 || observedPolicy.Type != vexocrypto.SignTypeConsensusVote || observedPolicy.Domain != vexocrypto.DomainConsensusVote {
		t.Fatalf("unexpected observed policy: %+v", observedPolicy)
	}
}

func TestRunKeysShowEncryptedRejectsWrongPassphrase(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--encrypt", "--passphrase", "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--home", home, "--passphrase", "wrong"}); err == nil {
		t.Fatal("expected wrong passphrase error")
	}
}

func TestRunKeysGenRejectsExistingUnlessOverwrite(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--type", "ed25519"}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err == nil {
		t.Fatal("expected keys gen to reject existing key")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--overwrite"}); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("expected overwrite to generate a new key")
	}
}

func TestRunKeysSupportsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.key.json")
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--path", path}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"show", "--path", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), path) {
		t.Fatalf("expected explicit path in output:\n%s", buffer.String())
	}
}

func TestRunKeysSignTx(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--type", "ed25519"}); err != nil {
		t.Fatal(err)
	}
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(home, ""))
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := decodeOptionalBase64(document.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	accountAddress, err := address.AccountFromPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runKeys(&output, []string{"sign-tx", "--home", home, "--chain-id", "vexo-test", "--tx", "bank:send:" + string(accountAddress) + ":bob:1:fee=1:gas=1:signer=" + string(accountAddress) + ":nonce=1"}); err != nil {
		t.Fatal(err)
	}
	signedTx := strings.TrimSpace(strings.TrimPrefix(output.String(), "tx: "))
	if !vexoapp.IsSignedTx([]byte(signedTx)) {
		t.Fatalf("expected signed tx output, got %s", output.String())
	}
	if err := vexoapp.VerifySignedTx("vexo-test", []byte(signedTx), vexocrypto.Ed25519Signer{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunKeysRejectsInvalidCommandsAndKeys(t *testing.T) {
	if err := runKeys(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected missing keys subcommand error")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"unknown"}); err == nil {
		t.Fatal("expected unknown keys subcommand error")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--home", t.TempDir()}); err == nil {
		t.Fatal("expected missing key error")
	}
	path := filepath.Join(t.TempDir(), keyFileName)
	if err := os.WriteFile(path, []byte(`{"schema_version":"v1","type":"ed25519","public_key":"bad","private_key":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--path", path}); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestResolveKeyPath(t *testing.T) {
	if got := resolveKeyPath("", "custom.key"); got != "custom.key" {
		t.Fatalf("expected explicit key path, got %q", got)
	}
	if got := resolveKeyPath("/tmp/vexo", ""); got != filepath.Join("/tmp/vexo", keyFileName) {
		t.Fatalf("unexpected key path: %q", got)
	}
}
