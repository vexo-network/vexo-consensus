package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	VRFAdapterRemoteHTTPName = "remote-vrf-http-v1"
	remoteVRFTokenEnv        = "VEXO_REMOTE_VRF_TOKEN"
)

var ErrMissingRemoteVRFURL = errors.New("remote vrf url is required")
var ErrRemoteVRFReplay = errors.New("remote vrf response failed nonce binding")
var ErrInvalidRemoteVRFTLS = errors.New("invalid remote vrf tls configuration")

type RemoteVRFAdapter struct {
	baseURL     string
	authToken   string
	client      *http.Client
	auditReport string
	dependency  string
	keySource   string
}

type remoteVRFProveRequest struct {
	PublicKey        string `json:"public_key"`
	Seed             string `json:"seed"`
	Nonce            string `json:"nonce"`
	IssuedAtUnixNano int64  `json:"issued_at_unix_nano"`
	DeadlineUnixNano int64  `json:"deadline_unix_nano"`
	Domain           string `json:"domain"`
}

type remoteVRFProveResponse struct {
	Output string `json:"output"`
	Proof  string `json:"proof"`
	Nonce  string `json:"nonce"`
}

type remoteVRFVerifyRequest struct {
	PublicKey        string `json:"public_key"`
	Seed             string `json:"seed"`
	Output           string `json:"output"`
	Proof            string `json:"proof"`
	Nonce            string `json:"nonce"`
	IssuedAtUnixNano int64  `json:"issued_at_unix_nano"`
	DeadlineUnixNano int64  `json:"deadline_unix_nano"`
	Domain           string `json:"domain"`
}

type remoteVRFVerifyResponse struct {
	Valid bool   `json:"valid"`
	Nonce string `json:"nonce"`
}

func init() {
	RegisterVRFAdapter(VRFAdapterRemoteHTTPName, NewRemoteVRFAdapter)
}

func NewRemoteVRFAdapter(cfg config.VRFConfig) (VRFAdapter, error) {
	baseURL := strings.TrimSpace(cfg.KeySource)
	baseURL = strings.TrimPrefix(baseURL, "remote-http:")
	if baseURL == "" {
		return nil, ErrMissingRemoteVRFURL
	}
	client, err := remoteVRFHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return RemoteVRFAdapter{
		baseURL:     strings.TrimRight(baseURL, "/"),
		authToken:   os.Getenv(remoteVRFTokenEnv),
		client:      client,
		auditReport: cfg.AuditReport,
		dependency:  cfg.DependencyAudit,
		keySource:   cfg.KeySource,
	}, nil
}

func remoteVRFHTTPClient(cfg config.VRFConfig) (*http.Client, error) {
	tlsConfigured := cfg.TLSCertPath != "" || cfg.TLSKeyPath != "" || cfg.TLSCAPath != "" || cfg.TLSServerName != ""
	if !tlsConfigured {
		return &http.Client{Timeout: 5 * time.Second}, nil
	}
	if (cfg.TLSCertPath == "") != (cfg.TLSKeyPath == "") {
		return nil, ErrInvalidRemoteVRFTLS
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.TLSServerName != "" {
		tlsConfig.ServerName = cfg.TLSServerName
	}
	if cfg.TLSCAPath != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRemoteVRFTLS, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, ErrInvalidRemoteVRFTLS
		}
		tlsConfig.RootCAs = pool
	}
	if cfg.TLSCertPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRemoteVRFTLS, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

func (adapter RemoteVRFAdapter) Prove(publicKey types.PublicKey, seed []byte) ([]byte, []byte, error) {
	ctx, cancel := adapter.defaultContext()
	defer cancel()
	return adapter.ProveWithContext(ctx, publicKey, seed)
}

func (adapter RemoteVRFAdapter) ProveWithContext(ctx context.Context, publicKey types.PublicKey, seed []byte) ([]byte, []byte, error) {
	challenge, err := newRemoteVRFChallenge()
	if err != nil {
		return nil, nil, err
	}
	var response remoteVRFProveResponse
	if err := adapter.post(ctx, "/prove", remoteVRFProveRequest{
		PublicKey:        base64.StdEncoding.EncodeToString(publicKey),
		Seed:             base64.StdEncoding.EncodeToString(seed),
		Nonce:            challenge.nonce,
		IssuedAtUnixNano: challenge.issuedAt,
		DeadlineUnixNano: challenge.deadline,
		Domain:           "vexo.remote_vrf.prove.v1",
	}, &response); err != nil {
		return nil, nil, err
	}
	if response.Nonce != challenge.nonce {
		return nil, nil, ErrRemoteVRFReplay
	}
	output, err := base64.StdEncoding.DecodeString(response.Output)
	if err != nil {
		return nil, nil, err
	}
	proof, err := base64.StdEncoding.DecodeString(response.Proof)
	if err != nil {
		return nil, nil, err
	}
	return output, proof, nil
}

func (adapter RemoteVRFAdapter) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	ctx, cancel := adapter.defaultContext()
	defer cancel()
	return adapter.VerifyWithContext(ctx, publicKey, seed, output, proof)
}

func (adapter RemoteVRFAdapter) VerifyWithContext(ctx context.Context, publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	challenge, err := newRemoteVRFChallenge()
	if err != nil {
		return false
	}
	var response remoteVRFVerifyResponse
	err = adapter.post(ctx, "/verify", remoteVRFVerifyRequest{
		PublicKey:        base64.StdEncoding.EncodeToString(publicKey),
		Seed:             base64.StdEncoding.EncodeToString(seed),
		Output:           base64.StdEncoding.EncodeToString(output),
		Proof:            base64.StdEncoding.EncodeToString(proof),
		Nonce:            challenge.nonce,
		IssuedAtUnixNano: challenge.issuedAt,
		DeadlineUnixNano: challenge.deadline,
		Domain:           "vexo.remote_vrf.verify.v1",
	}, &response)
	return err == nil && response.Valid && response.Nonce == challenge.nonce
}

func (adapter RemoteVRFAdapter) defaultContext() (context.Context, context.CancelFunc) {
	timeout := 5 * time.Second
	if adapter.client != nil && adapter.client.Timeout > 0 {
		timeout = adapter.client.Timeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (adapter RemoteVRFAdapter) Metadata() VRFAdapterMetadata {
	return VRFAdapterMetadata{
		Name:                 VRFAdapterRemoteHTTPName,
		Version:              "v1",
		Audited:              adapter.auditReport != "",
		AuditReport:          adapter.auditReport,
		DependencyAudit:      adapter.dependency,
		KeySource:            adapter.keySource,
		DomainSeparation:     true,
		ProofVerification:    true,
		DeterministicOutput:  true,
		MalformedInputFuzzed: adapter.auditReport != "",
	}
}

func (adapter RemoteVRFAdapter) post(ctx context.Context, path string, requestBody any, responseBody any) error {
	if adapter.baseURL == "" {
		return ErrMissingRemoteVRFURL
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if adapter.authToken != "" {
		request.Header.Set("Authorization", "Bearer "+adapter.authToken)
	}
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status=%d", ErrRemoteSignerRejected, response.StatusCode)
	}
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(responseBody)
}

type remoteVRFChallenge struct {
	nonce    string
	issuedAt int64
	deadline int64
}

func newRemoteVRFChallenge() (remoteVRFChallenge, error) {
	var nonce [32]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return remoteVRFChallenge{}, err
	}
	now := time.Now()
	return remoteVRFChallenge{
		nonce:    base64.RawURLEncoding.EncodeToString(nonce[:]),
		issuedAt: now.UnixNano(),
		deadline: now.Add(5 * time.Second).UnixNano(),
	}, nil
}
