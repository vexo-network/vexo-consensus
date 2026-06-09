package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

type RemoteVRFAdapter struct {
	baseURL     string
	authToken   string
	client      *http.Client
	auditReport string
	keySource   string
}

type remoteVRFProveRequest struct {
	PublicKey string `json:"public_key"`
	Seed      string `json:"seed"`
}

type remoteVRFProveResponse struct {
	Output string `json:"output"`
	Proof  string `json:"proof"`
}

type remoteVRFVerifyRequest struct {
	PublicKey string `json:"public_key"`
	Seed      string `json:"seed"`
	Output    string `json:"output"`
	Proof     string `json:"proof"`
}

type remoteVRFVerifyResponse struct {
	Valid bool `json:"valid"`
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
	return RemoteVRFAdapter{
		baseURL:     strings.TrimRight(baseURL, "/"),
		authToken:   os.Getenv(remoteVRFTokenEnv),
		client:      &http.Client{Timeout: 5 * time.Second},
		auditReport: cfg.AuditReport,
		keySource:   cfg.KeySource,
	}, nil
}

func (adapter RemoteVRFAdapter) Prove(publicKey types.PublicKey, seed []byte) ([]byte, []byte, error) {
	var response remoteVRFProveResponse
	if err := adapter.post(context.Background(), "/prove", remoteVRFProveRequest{
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		Seed:      base64.StdEncoding.EncodeToString(seed),
	}, &response); err != nil {
		return nil, nil, err
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
	var response remoteVRFVerifyResponse
	err := adapter.post(context.Background(), "/verify", remoteVRFVerifyRequest{
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		Seed:      base64.StdEncoding.EncodeToString(seed),
		Output:    base64.StdEncoding.EncodeToString(output),
		Proof:     base64.StdEncoding.EncodeToString(proof),
	}, &response)
	return err == nil && response.Valid
}

func (adapter RemoteVRFAdapter) Metadata() VRFAdapterMetadata {
	return VRFAdapterMetadata{
		Name:                 VRFAdapterRemoteHTTPName,
		Version:              "v1",
		Audited:              adapter.auditReport != "",
		AuditReport:          adapter.auditReport,
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
