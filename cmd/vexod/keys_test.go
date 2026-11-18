package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/vexo-network/vexo-consensus/mempool"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
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
	if !strings.Contains(output.String(), "remote signer verified") || !strings.Contains(output.String(), "signature:") || !strings.Contains(output.String(), "policy: "+defaultChainID+"/1/remote_signer_probe/vexo.remote_signer.probe.v1") {
		t.Fatalf("unexpected verify remote output:\n%s", output.String())
	}
	if observedPolicy == nil || observedPolicy.ChainID != defaultChainID || observedPolicy.Height != 1 || observedPolicy.Type != vexocrypto.SignTypeRemoteSignerProbe || observedPolicy.Domain != vexocrypto.DomainRemoteSignerProbe {
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

// generateSigningKey creates an Ed25519 key in a fresh home and returns the home with the account
// address of the key.
func generateSigningKey(t *testing.T) (home string, account string) {
	t.Helper()
	home = t.TempDir()
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
	return home, string(accountAddress)
}

// signTxFields runs keys sign-tx for chain vexo-test and returns the fields it prints by name.
func signTxFields(t *testing.T, home string, payload string) map[string]string {
	t.Helper()
	var output bytes.Buffer
	if err := runKeys(&output, []string{"sign-tx", "--home", home, "--chain-id", "vexo-test", "--tx", payload}); err != nil {
		t.Fatal(err)
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		name, value, found := strings.Cut(line, ": ")
		if !found {
			t.Fatalf("unexpected keys sign-tx line %q in:\n%s", line, output.String())
		}
		fields[name] = value
	}
	return fields
}

func TestRunKeysSignTx(t *testing.T) {
	home, account := generateSigningKey(t)
	payload := "bank:send:" + account + ":bob:1:fee=1:gas=1:signer=" + account + ":nonce=1"
	fields := signTxFields(t, home, payload)
	if len(fields) != 2 || fields["encoding"] != txcodec.EncodingBase64 {
		t.Fatalf("expected the transaction and the encoding it is printed in, got %v", fields)
	}
	// The envelope is binary: what is printed is its base64, never the bytes themselves.
	signedTx, err := base64.StdEncoding.DecodeString(fields["tx"])
	if err != nil {
		t.Fatalf("expected the signed tx to be printed as base64: %v", err)
	}
	if !vexoapp.IsSignedTx(signedTx) {
		t.Fatalf("expected signed tx output, got %v", fields)
	}
	if err := vexoapp.VerifySignedTx("vexo-test", signedTx, vexocrypto.Ed25519Signer{}); err != nil {
		t.Fatal(err)
	}
	if string(vexoapp.TxPayload(signedTx)) != payload {
		t.Fatalf("unexpected signed payload %q", vexoapp.TxPayload(signedTx))
	}
}

// txSink is the smallest RPC provider that takes transactions: it keeps what the submit handler
// decoded from a request.
type txSink struct {
	submitted []types.Tx
}

func (sink *txSink) Status(context.Context) vexonode.Status { return vexonode.Status{} }

func (sink *txSink) SubmitTx(_ context.Context, tx types.Tx) error {
	sink.submitted = append(sink.submitted, bytes.Clone(tx))
	return nil
}

// submitTxRequest is the request that submits body to the RPC transaction endpoint.
func submitTxRequest(body []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/v1/tx", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// What keys sign-tx prints is what the RPC submit endpoint decodes: the transaction that reaches
// the node is the signed one, byte for byte, whether the request names the printed encoding or
// leaves it to the default.
func TestKeysSignTxOutputGoesThroughTheRPCDecodePath(t *testing.T) {
	home, account := generateSigningKey(t)
	payload := "bank:send:" + account + ":bob:1:fee=1:gas=1:signer=" + account + ":nonce=1"
	fields := signTxFields(t, home, payload)
	want, err := base64.StdEncoding.DecodeString(fields["tx"])
	if err != nil {
		t.Fatal(err)
	}
	wantHash := mempool.HashTx(want)

	for name, request := range map[string]map[string]string{
		"printed encoding": {"tx": fields["tx"], "encoding": fields["encoding"]},
		"default encoding": {"tx": fields["tx"]},
	} {
		sink := &txSink{}
		body, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		vexorpc.NewHandler(sink).ServeHTTP(response, submitTxRequest(body))
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s: expected the signed tx to be accepted, got HTTP %d: %s", name, response.Code, response.Body.String())
		}
		if len(sink.submitted) != 1 || !bytes.Equal(sink.submitted[0], want) {
			t.Fatalf("%s: the node received %x instead of %x", name, sink.submitted, want)
		}
		var accepted vexorpc.SubmitTxResponse
		if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
			t.Fatal(err)
		}
		if !accepted.Accepted || accepted.TxHash != hex.EncodeToString(wantHash[:]) {
			t.Fatalf("%s: unexpected submit response %+v", name, accepted)
		}
		if err := vexoapp.VerifySignedTx("vexo-test", sink.submitted[0], vexocrypto.Ed25519Signer{}); err != nil {
			t.Fatalf("%s: the submitted tx does not verify: %v", name, err)
		}
	}

	// Text that is not base64 does not get to the node: the signed tx is binary, and plain would
	// hand the node the characters of the base64.
	sink := &txSink{}
	response := httptest.NewRecorder()
	vexorpc.NewHandler(sink).ServeHTTP(response, submitTxRequest([]byte(`{"tx":"not base64!"}`)))
	if response.Code != http.StatusBadRequest || len(sink.submitted) != 0 {
		t.Fatalf("expected invalid base64 to be rejected, got HTTP %d with %d submitted", response.Code, len(sink.submitted))
	}
}

// The submission the network commands make for their smoke and load transactions is the one the
// RPC handler decodes.
func TestSubmitNetworkTxCarriesSignedTxThroughTheRPCDecodePath(t *testing.T) {
	home, account := generateSigningKey(t)
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(home, ""))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := document.SignerWithPassphrase("")
	if err != nil {
		t.Fatal(err)
	}
	signedTx, err := vexoapp.SignTx("vexo-test", types.Tx("bank:send:"+account+":bob:1:fee=1:gas=1:signer="+account+":nonce=1"), signer)
	if err != nil {
		t.Fatal(err)
	}

	sink := &txSink{}
	server := httptest.NewServer(vexorpc.NewHandler(sink))
	defer server.Close()
	if err := submitNetworkTx(context.Background(), http.Client{}, strings.TrimPrefix(server.URL, "http://"), signedTx); err != nil {
		t.Fatal(err)
	}
	if len(sink.submitted) != 1 || !bytes.Equal(sink.submitted[0], signedTx) {
		t.Fatalf("the node received %x instead of %x", sink.submitted, []byte(signedTx))
	}
}

// keys sign-tx prints the base64 of a binary transaction, and tx parse reads that base64 back
// when it is told so; the default stays the plain text that tx build prints.
func TestTxParseReadsTheSignedTxThatKeysSignTxPrints(t *testing.T) {
	home, account := generateSigningKey(t)
	payload := "bank:send:" + account + ":bob:1:fee=1:gas=1:signer=" + account + ":nonce=1"
	fields := signTxFields(t, home, payload)

	var parsed bytes.Buffer
	if err := runCommand(&parsed, &bytes.Buffer{}, []string{"tx", "parse", "--tx", fields["tx"], "--encoding", fields["encoding"], "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"module": "bank"`, `"action": "send"`, `"nonce": "1"`, `"signer": "` + account + `"`} {
		if !strings.Contains(parsed.String(), expected) {
			t.Fatalf("expected tx parse to read the signed payload, missing %s in:\n%s", expected, parsed.String())
		}
	}
	parsed.Reset()
	if err := runCommand(&parsed, &bytes.Buffer{}, []string{"tx", "parse", "--tx", fields["tx"], "--encoding", fields["encoding"]}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.String(), "module: bank\n") || !strings.Contains(parsed.String(), "nonce: 1\n") {
		t.Fatalf("unexpected tx parse output:\n%s", parsed.String())
	}

	// Without the encoding the base64 is a plain payload, which is not a transaction.
	if err := runCommand(&bytes.Buffer{}, &bytes.Buffer{}, []string{"tx", "parse", "--tx", fields["tx"]}); err == nil {
		t.Fatal("expected the base64 of a signed tx to be rejected as a plain payload")
	}
	for name, args := range map[string][]string{
		"unknown encoding": {"tx", "parse", "--tx", fields["tx"], "--encoding", "hex"},
		"invalid base64":   {"tx", "parse", "--tx", "not base64!", "--encoding", "base64"},
		"truncated signed": {"tx", "parse", "--tx", base64.StdEncoding.EncodeToString([]byte(txcodec.SignedPrefix + "\x01")), "--encoding", "base64"},
	} {
		if err := runCommand(&bytes.Buffer{}, &bytes.Buffer{}, args); err == nil {
			t.Fatalf("%s: expected tx parse to fail", name)
		}
	}
	var unknown bytes.Buffer
	err := runCommand(&unknown, &bytes.Buffer{}, []string{"tx", "parse", "--tx", "bank:send:alice:bob:1", "--encoding", "hex"})
	if err == nil || !strings.Contains(err.Error(), `unsupported transaction encoding "hex"`) {
		t.Fatalf("expected an unsupported encoding error, got %v", err)
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
