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
	"sort"
	"time"

	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	keyFileName     = "validator.key.json"
	nodeKeyFileName = "node.key.json"
)

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

type keyRotationPlanDocument struct {
	SchemaVersion string                `json:"schema_version"`
	OK            bool                  `json:"ok"`
	Keys          []keyRotationPlanItem `json:"keys"`
	Gaps          []string              `json:"gaps,omitempty"`
	Overlaps      []string              `json:"overlaps,omitempty"`
}

type keyRotationPlanItem struct {
	Path        string `json:"path"`
	KeyID       string `json:"key_id"`
	Type        string `json:"type"`
	Address     string `json:"address"`
	PublicKey   string `json:"public_key"`
	ActiveFrom  uint64 `json:"active_from"`
	ActiveUntil uint64 `json:"active_until,omitempty"`
	RemoteURL   string `json:"remote_url,omitempty"`
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
	case "serve-vrf":
		return runKeysServeVRF(writer, args[1:])
	case "verify-vrf":
		return runKeysVerifyVRF(writer, args[1:])
	case "rotation-plan":
		return runKeysRotationPlan(writer, args[1:])
	default:
		return fmt.Errorf("unknown keys subcommand %q", args[0])
	}
}

func runKeysGen(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	keyType := flags.String("type", vexocrypto.KeyTypeBLS, "key type: bls, ed25519, or vrf")
	blsAdapter := flags.String("bls-adapter", vexocrypto.BLSAdapterBLSTName, "BLS adapter for --type bls")
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
	document, err := generateKeyDocumentWithBLSAdapter(*keyType, *blsAdapter)
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
	if err := maybeUpdateGenesisValidatorKey(*home, keyPath, document); err != nil {
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
	return generateKeyDocumentWithBLSAdapter(keyType, vexocrypto.BLSAdapterBLSTName)
}

func generateKeyDocumentWithBLSAdapter(keyType string, blsAdapterName string) (vexocrypto.KeyDocument, error) {
	switch keyType {
	case vexocrypto.KeyTypeEd25519:
		return vexocrypto.GenerateEd25519KeyDocument()
	case vexocrypto.KeyTypeBLS:
		if blsAdapterName == "" || blsAdapterName == vexocrypto.BLSAdapterBLSTName {
			adapter, err := vexocrypto.GenerateBLSTBLSAdapter()
			if err != nil {
				return vexocrypto.KeyDocument{}, err
			}
			return vexocrypto.NewBLSTBLSKeyDocument(adapter)
		}
		if blsAdapterName != vexocrypto.BLSAdapterCIRCLName {
			return vexocrypto.KeyDocument{}, vexocrypto.ErrUnsupportedSignerBackend
		}
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
	if err := maybeUpdateGenesisValidatorKey(*home, keyPath, document); err != nil {
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

func maybeUpdateGenesisValidatorKey(home string, keyPath string, document vexocrypto.KeyDocument) error {
	if filepath.Clean(keyPath) != filepath.Clean(resolveKeyPath(home, "")) {
		return nil
	}
	switch document.Type {
	case vexocrypto.KeyTypeEd25519, vexocrypto.KeyTypeBLS, vexocrypto.KeyTypeRemote:
	default:
		return nil
	}
	cfg, err := loadNodeConfig(resolveConfigPath(home, ""))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if cfg.ValidatorID == "" {
		return nil
	}
	genesisPath := resolveGenesisPath(home, "")
	genesis, err := readGenesisDocument(genesisPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := applyValidatorKeyToGenesisDocument(&genesis, string(cfg.ValidatorID), document); err != nil {
		return err
	}
	return writeJSONFile(genesisPath, genesis)
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
	noncePath := flags.String("nonce-path", "", "replay nonce guard file path")
	authToken := flags.String("auth-token", "", "required bearer token for remote signing requests")
	authTokenEnv := flags.String("auth-token-env", "", "environment variable containing required bearer token")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *guardPath == "" {
		*guardPath = filepath.Join(*home, "remote-signer.guard.json")
	}
	if *noncePath == "" {
		*noncePath = filepath.Join(*home, "remote-signer.nonces.json")
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
	nonceGuard, err := vexocrypto.NewFileBackedRemoteSignerNonceGuard(*noncePath)
	if err != nil {
		return err
	}
	service, err := vexocrypto.NewRemoteSignerServiceWithNonceGuard(signer, vexocrypto.RemoteSignerPolicy{
		ChainID:       *chainID,
		MinHeight:     types.Height(*minHeight),
		MaxHeight:     types.Height(*maxHeight),
		AllowedTypes:  []vexocrypto.SignType{vexocrypto.SignTypeConsensusProposal, vexocrypto.SignTypeConsensusVote, vexocrypto.SignTypeConsensusTimeoutVote, vexocrypto.SignTypeFinalityProof},
		RequirePolicy: true,
		AuthToken:     resolvedAuthToken,
	}, guard, nonceGuard)
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
	fmt.Fprintf(writer, "nonce_path: %s\n", *noncePath)
	fmt.Fprintf(writer, "auth_required: %t\n", resolvedAuthToken != "")
	return server.ListenAndServe()
}

func runKeysServeVRF(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys serve-vrf", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "local VRF key file path")
	listen := flags.String("listen", "127.0.0.1:9100", "HTTP listen address")
	authToken := flags.String("auth-token", "", "required bearer token for remote VRF requests")
	authTokenEnv := flags.String("auth-token-env", "", "environment variable containing required bearer token")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	resolvedAuthToken := *authToken
	if resolvedAuthToken == "" && *authTokenEnv != "" {
		resolvedAuthToken = os.Getenv(*authTokenEnv)
	}
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(*home, *path))
	if err != nil {
		return err
	}
	privateKey, err := document.ECVRFP256PrivateKeyWithPassphrase(resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	publicKey, err := vexocrypto.ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		return err
	}
	vrf, err := vexocrypto.NewECVRFP256Adapter(config.VRFConfig{
		Keys: map[string][]byte{
			string(publicKey): privateKey,
			base64.StdEncoding.EncodeToString(publicKey): privateKey,
		},
		AuditReport:     "local-ecvrf-remote-service",
		DependencyAudit: config.NetworkSafeVRFDependencyAudit,
		KeySource:       "keys.serve-vrf",
	})
	if err != nil {
		return err
	}
	service, err := vexocrypto.NewRemoteVRFService(vrf, vexocrypto.RemoteVRFServiceConfig{AuthToken: resolvedAuthToken})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           service,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(writer, "remote vrf serving\n")
	fmt.Fprintf(writer, "listen: %s\n", *listen)
	fmt.Fprintf(writer, "public_key: %s\n", base64.StdEncoding.EncodeToString(publicKey))
	fmt.Fprintf(writer, "auth_required: %t\n", resolvedAuthToken != "")
	return server.ListenAndServe()
}

func runKeysVerifyVRF(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys verify-vrf", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	url := flags.String("url", "", "remote VRF base URL")
	publicKeyString := flags.String("public-key", "", "base64 encoded VRF public key")
	seedString := flags.String("seed", "vexo-vrf-check", "seed string to prove and verify")
	authToken := flags.String("auth-token", "", "bearer token for remote VRF requests")
	authTokenEnv := flags.String("auth-token-env", "", "environment variable containing bearer token")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *url == "" {
		return errors.New("remote VRF URL is required")
	}
	if *publicKeyString == "" {
		return errors.New("remote VRF public key is required")
	}
	publicKey, err := base64.StdEncoding.DecodeString(*publicKeyString)
	if err != nil {
		return fmt.Errorf("invalid remote VRF public key: %w", err)
	}
	resolvedAuthToken := *authToken
	if resolvedAuthToken == "" && *authTokenEnv != "" {
		resolvedAuthToken = os.Getenv(*authTokenEnv)
	}
	adapter, err := vexocrypto.NewRemoteVRFAdapterWithAuth(config.VRFConfig{
		AdapterName:     vexocrypto.VRFAdapterRemoteHTTPName,
		AuditReport:     "remote-vrf-operator-check",
		DependencyAudit: "external:remote-vrf-service",
		KeySource:       "remote-http:" + *url,
	}, resolvedAuthToken)
	if err != nil {
		return err
	}
	output, proof, err := adapter.Prove(types.PublicKey(publicKey), []byte(*seedString))
	if err != nil {
		return err
	}
	verified := adapter.Verify(types.PublicKey(publicKey), []byte(*seedString), output, proof)
	if !verified {
		return errors.New("remote VRF proof verification failed")
	}
	result := map[string]any{
		"ok":         true,
		"remote_url": *url,
		"public_key": *publicKeyString,
		"seed":       *seedString,
		"output":     base64.StdEncoding.EncodeToString(output),
		"proof":      base64.StdEncoding.EncodeToString(proof),
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	fmt.Fprintf(writer, "remote vrf verified\n")
	fmt.Fprintf(writer, "remote_url: %s\n", *url)
	fmt.Fprintf(writer, "public_key: %s\n", *publicKeyString)
	fmt.Fprintf(writer, "output: %s\n", base64.StdEncoding.EncodeToString(output))
	fmt.Fprintf(writer, "proof: %s\n", base64.StdEncoding.EncodeToString(proof))
	return nil
}

func runKeysRotationPlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys rotation-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	keyPaths := stringListFlags{}
	flags.Var(&keyPaths, "key", "key file path; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(keyPaths) == 0 {
		keyPaths = append(keyPaths, resolveKeyPath(*home, ""))
	}
	plan, err := buildKeyRotationPlan(*home, []string(keyPaths), resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	fmt.Fprintf(writer, "key rotation plan\n")
	fmt.Fprintf(writer, "ok: %t\n", plan.OK)
	for _, item := range plan.Keys {
		fmt.Fprintf(writer, "- %s id=%s type=%s active_from=%d active_until=%d address=%s\n", item.Path, item.KeyID, item.Type, item.ActiveFrom, item.ActiveUntil, item.Address)
	}
	for _, gap := range plan.Gaps {
		fmt.Fprintf(writer, "gap: %s\n", gap)
	}
	for _, overlap := range plan.Overlaps {
		fmt.Fprintf(writer, "overlap: %s\n", overlap)
	}
	return nil
}

func buildKeyRotationPlan(home string, keyPaths []string, passphrase string) (keyRotationPlanDocument, error) {
	items := make([]keyRotationPlanItem, 0, len(keyPaths))
	for _, keyPath := range keyPaths {
		resolvedPath := resolveRotationKeyPath(home, keyPath)
		document, err := vexocrypto.LoadKeyDocument(resolvedPath)
		if err != nil {
			return keyRotationPlanDocument{}, err
		}
		record, err := document.KeyRecordWithPassphrase(passphrase)
		if err != nil {
			return keyRotationPlanDocument{}, err
		}
		accountAddress, err := keyDocumentAccountAddress(document)
		if err != nil {
			return keyRotationPlanDocument{}, err
		}
		items = append(items, keyRotationPlanItem{
			Path:        resolvedPath,
			KeyID:       string(record.ID),
			Type:        document.Type,
			Address:     string(accountAddress),
			PublicKey:   document.PublicKey,
			ActiveFrom:  record.ActiveFrom,
			ActiveUntil: record.ActiveUntil,
			RemoteURL:   document.Metadata.RemoteURL,
		})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].ActiveFrom == items[right].ActiveFrom {
			return items[left].KeyID < items[right].KeyID
		}
		return items[left].ActiveFrom < items[right].ActiveFrom
	})
	plan := keyRotationPlanDocument{
		SchemaVersion: vexocrypto.KeyDocumentVersionV1,
		OK:            true,
		Keys:          items,
	}
	for index := 1; index < len(items); index++ {
		previous := items[index-1]
		current := items[index]
		switch {
		case previous.ActiveUntil == 0:
			plan.Overlaps = append(plan.Overlaps, fmt.Sprintf("%s has no active_until before %s starts at %d", previous.KeyID, current.KeyID, current.ActiveFrom))
		case current.ActiveFrom <= previous.ActiveUntil:
			plan.Overlaps = append(plan.Overlaps, fmt.Sprintf("%s starts at %d before %s ends at %d", current.KeyID, current.ActiveFrom, previous.KeyID, previous.ActiveUntil))
		case previous.ActiveUntil+1 < current.ActiveFrom:
			plan.Gaps = append(plan.Gaps, fmt.Sprintf("no active key between heights %d and %d", previous.ActiveUntil+1, current.ActiveFrom-1))
		}
	}
	plan.OK = len(plan.Gaps) == 0 && len(plan.Overlaps) == 0
	return plan, nil
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
