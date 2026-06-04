package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
)

const keyFileName = "validator.key.json"

type keyInfoDocument struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	Address       string `json:"address"`
	PublicKey     string `json:"public_key"`
	Path          string `json:"path"`
	Encrypted     bool   `json:"encrypted"`
	KeyID         string `json:"key_id,omitempty"`
	ActiveFrom    uint64 `json:"active_from,omitempty"`
	ActiveUntil   uint64 `json:"active_until,omitempty"`
	RemoteURL     string `json:"remote_url,omitempty"`
}

func runKeys(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("keys subcommand is required")
	}
	switch args[0] {
	case "gen":
		return runKeysGen(writer, args[1:])
	case "remote":
		return runKeysRemote(writer, args[1:])
	case "show":
		return runKeysShow(writer, args[1:])
	case "sign-tx":
		return runKeysSignTx(writer, args[1:])
	case "verify-remote":
		return runKeysVerifyRemote(writer, args[1:])
	case "serve-remote":
		return runKeysServeRemote(writer, args[1:])
	default:
		return fmt.Errorf("unknown keys subcommand %q", args[0])
	}
}

func runKeysGen(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	keyType := flags.String("type", vexocrypto.KeyTypeEd25519, "key type: ed25519, bls, or vrf")
	overwrite := flags.Bool("overwrite", false, "overwrite existing key")
	encrypt := flags.Bool("encrypt", false, "encrypt private key material")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	keyID := flags.String("id", "", "key id metadata")
	activeFrom := flags.Uint64("active-from", 0, "first height where key is active")
	activeUntil := flags.Uint64("active-until", 0, "last height where key is active; zero means no end")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return err
	}
	if *overwrite {
		if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	document, err := generateKeyDocument(*keyType)
	if err != nil {
		return err
	}
	document.Metadata.ID = *keyID
	document.Metadata.ActiveFrom = *activeFrom
	document.Metadata.ActiveUntil = *activeUntil
	if *encrypt {
		encrypted, err := document.Encrypted(resolvePassphrase(*passphrase))
		if err != nil {
			return err
		}
		document = encrypted
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, document); err != nil {
		return err
	}
	accountAddress, err := keyDocumentAccountAddress(document)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "generated validator key\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "type: %s\n", document.Type)
	fmt.Fprintf(writer, "address: %s\n", accountAddress)
	fmt.Fprintf(writer, "public_key: %s\n", document.PublicKey)
	fmt.Fprintf(writer, "encrypted: %v\n", document.Encryption != nil)
	return nil
}

func generateKeyDocument(keyType string) (vexocrypto.KeyDocument, error) {
	switch keyType {
	case vexocrypto.KeyTypeEd25519:
		return vexocrypto.GenerateEd25519KeyDocument()
	case vexocrypto.KeyTypeBLS:
		adapter, err := vexocrypto.GenerateCIRCLBLSAdapter()
		if err != nil {
			return vexocrypto.KeyDocument{}, err
		}
		return vexocrypto.NewCIRCLBLSKeyDocument(adapter)
	case vexocrypto.KeyTypeVRF:
		return vexocrypto.GenerateECVRFP256KeyDocument()
	default:
		return vexocrypto.KeyDocument{}, vexocrypto.ErrUnsupportedKeyType
	}
}

func runKeysRemote(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	overwrite := flags.Bool("overwrite", false, "overwrite existing key")
	publicKey := flags.String("public-key", "", "base64 encoded remote signer public key")
	url := flags.String("url", "", "remote signer HTTP endpoint")
	keyID := flags.String("id", "", "key id metadata")
	activeFrom := flags.Uint64("active-from", 0, "first height where key is active")
	activeUntil := flags.Uint64("active-until", 0, "last height where key is active; zero means no end")
	authTokenEnv := flags.String("auth-token-env", "", "environment variable containing remote signer bearer token")
	guardPath := flags.String("guard-path", "", "expected remote signer double-sign guard path metadata")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *publicKey == "" {
		return errors.New("remote public key is required")
	}
	if *url == "" {
		return errors.New("remote signer URL is required")
	}
	keyPath := resolveKeyPath(*home, *path)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return err
	}
	if *overwrite {
		if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	document := vexocrypto.KeyDocument{
		SchemaVersion: vexocrypto.KeyDocumentVersionV1,
		Type:          vexocrypto.KeyTypeRemote,
		PublicKey:     *publicKey,
		Metadata: vexocrypto.KeyMetadata{
			ID:            *keyID,
			ActiveFrom:    *activeFrom,
			ActiveUntil:   *activeUntil,
			RemoteURL:     *url,
			AuthTokenEnv:  *authTokenEnv,
			RequirePolicy: true,
			GuardPath:     *guardPath,
		},
	}
	if _, err := document.RemoteSigner(0); err != nil {
		return err
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, document); err != nil {
		return err
	}
	accountAddress, err := keyDocumentAccountAddress(document)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "registered remote validator key\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "type: %s\n", document.Type)
	fmt.Fprintf(writer, "address: %s\n", accountAddress)
	fmt.Fprintf(writer, "public_key: %s\n", document.PublicKey)
	fmt.Fprintf(writer, "remote_url: %s\n", document.Metadata.RemoteURL)
	if document.Metadata.AuthTokenEnv != "" {
		fmt.Fprintf(writer, "auth_token_env: %s\n", document.Metadata.AuthTokenEnv)
	}
	return nil
}

func runKeysShow(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		return err
	}
	if err := validateKeyDocumentForShow(document, resolvePassphrase(*passphrase)); err != nil {
		return err
	}
	info := keyInfoDocument{
		SchemaVersion: document.SchemaVersion,
		Type:          document.Type,
		Address:       "",
		PublicKey:     document.PublicKey,
		Path:          keyPath,
		Encrypted:     document.Encryption != nil,
		KeyID:         document.Metadata.ID,
		ActiveFrom:    document.Metadata.ActiveFrom,
		ActiveUntil:   document.Metadata.ActiveUntil,
		RemoteURL:     document.Metadata.RemoteURL,
	}
	accountAddress, err := keyDocumentAccountAddress(document)
	if err != nil {
		return err
	}
	info.Address = string(accountAddress)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(info)
	}
	fmt.Fprintf(writer, "validator key\n")
	fmt.Fprintf(writer, "path: %s\n", info.Path)
	fmt.Fprintf(writer, "type: %s\n", info.Type)
	fmt.Fprintf(writer, "address: %s\n", info.Address)
	fmt.Fprintf(writer, "public_key: %s\n", info.PublicKey)
	fmt.Fprintf(writer, "encrypted: %v\n", info.Encrypted)
	if info.RemoteURL != "" {
		fmt.Fprintf(writer, "remote_url: %s\n", info.RemoteURL)
	}
	return nil
}

func validateKeyDocumentForShow(document vexocrypto.KeyDocument, passphrase string) error {
	switch document.Type {
	case vexocrypto.KeyTypeVRF:
		_, err := document.ECVRFP256PrivateKeyWithPassphrase(passphrase)
		return err
	default:
		_, err := document.SignerWithPassphrase(passphrase)
		return err
	}
}

func keyDocumentAccountAddress(document vexocrypto.KeyDocument) (types.Address, error) {
	publicKey, err := decodeOptionalBase64(document.PublicKey)
	if err != nil {
		return "", err
	}
	return address.AccountFromPublicKey(publicKey)
}

func runKeysSignTx(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys sign-tx", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	chainID := flags.String("chain-id", defaultChainID, "chain id for signed transaction domain")
	tx := flags.String("tx", "", "raw transaction payload to sign")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *tx == "" {
		return errors.New("transaction payload is required")
	}
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(*home, *path))
	if err != nil {
		return err
	}
	signer, err := document.SignerWithPassphrase(resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	signedTx, err := vexoapp.SignTx(*chainID, types.Tx(*tx), signer)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", signedTx)
	return nil
}

func runKeysVerifyRemote(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys verify-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	challenge := flags.String("challenge", "vexo-remote-signer-check", "challenge message to sign")
	chainID := flags.String("chain-id", defaultChainID, "chain id for remote signer policy")
	height := flags.Uint64("height", 1, "height for remote signer policy")
	round := flags.Uint64("round", 0, "round for remote signer policy")
	signType := flags.String("type", string(vexocrypto.SignTypeConsensusVote), "sign type for remote signer policy")
	domain := flags.String("domain", string(vexocrypto.DomainConsensusVote), "signature domain for remote signer policy")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		return err
	}
	signer, err := document.RemoteSigner(5 * time.Second)
	if err != nil {
		return err
	}
	message := []byte(*challenge)
	policy := vexocrypto.SignPolicy{
		ChainID: *chainID,
		Height:  types.Height(*height),
		Round:   types.Round(*round),
		Type:    vexocrypto.SignType(*signType),
		Domain:  vexocrypto.Domain(*domain),
	}
	signature, err := signer.SignWithPolicy(policy, message)
	if err != nil {
		return err
	}
	verified := signer.Verify(signer.PublicKey(), message, signature)
	if !verified {
		return errors.New("remote signer signature verification failed")
	}
	result := map[string]any{
		"ok":         true,
		"path":       keyPath,
		"remote_url": document.Metadata.RemoteURL,
		"signature":  base64.StdEncoding.EncodeToString(signature),
		"policy":     policy,
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	fmt.Fprintf(writer, "remote signer verified\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "remote_url: %s\n", document.Metadata.RemoteURL)
	fmt.Fprintf(writer, "policy: %s\n", policy.GuardKey())
	fmt.Fprintf(writer, "signature: %s\n", base64.StdEncoding.EncodeToString(signature))
	return nil
}

func runKeysServeRemote(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys serve-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "local validator key file path")
	listen := flags.String("listen", "127.0.0.1:9000", "HTTP listen address")
	chainID := flags.String("chain-id", defaultChainID, "allowed chain id")
	minHeight := flags.Uint64("min-height", 0, "minimum allowed signing height")
	maxHeight := flags.Uint64("max-height", 0, "maximum allowed signing height; zero means no limit")
	guardPath := flags.String("guard-path", "", "double-sign guard file path")
	authToken := flags.String("auth-token", "", "required bearer token for remote signing requests")
	authTokenEnv := flags.String("auth-token-env", "", "environment variable containing required bearer token")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *guardPath == "" {
		*guardPath = filepath.Join(*home, "remote-signer.guard.json")
	}
	resolvedAuthToken := *authToken
	if resolvedAuthToken == "" && *authTokenEnv != "" {
		resolvedAuthToken = os.Getenv(*authTokenEnv)
	}
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(*home, *path))
	if err != nil {
		return err
	}
	signer, err := document.SignerWithPassphrase(resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	guard, err := vexocrypto.NewFileBackedDoubleSignGuard(*guardPath)
	if err != nil {
		return err
	}
	service, err := vexocrypto.NewRemoteSignerService(signer, vexocrypto.RemoteSignerPolicy{
		ChainID:       *chainID,
		MinHeight:     types.Height(*minHeight),
		MaxHeight:     types.Height(*maxHeight),
		AllowedTypes:  []vexocrypto.SignType{vexocrypto.SignTypeConsensusProposal, vexocrypto.SignTypeConsensusVote, vexocrypto.SignTypeConsensusTimeoutVote, vexocrypto.SignTypeFinalityProof},
		RequirePolicy: true,
		AuthToken:     resolvedAuthToken,
	}, guard)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           service,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(writer, "remote signer serving\n")
	fmt.Fprintf(writer, "listen: %s\n", *listen)
	fmt.Fprintf(writer, "chain_id: %s\n", *chainID)
	fmt.Fprintf(writer, "guard_path: %s\n", *guardPath)
	fmt.Fprintf(writer, "auth_required: %t\n", resolvedAuthToken != "")
	return server.ListenAndServe()
}

func resolvePassphrase(passphrase string) string {
	if passphrase != "" {
		return passphrase
	}
	return os.Getenv("VEXO_KEY_PASSPHRASE")
}

func resolveKeyPath(home string, path string) string {
	if path != "" {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, keyFileName)
}
