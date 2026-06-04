package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"math/big"

	ecvrf "github.com/vechain/go-ecvrf"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	VRFAdapterECVRFP256Name   = "ecvrf-p256-sha256-tai-v1"
	ecvrfDomain               = "VEXO_ECVRF_P256_SHA256_TAI_V1:"
	ecvrfDependencyTag        = "github.com/vechain/go-ecvrf@v0.0.0-20251211112124-5d5a3ef70fc9"
	ecvrfDefaultKeySourceName = "config.vrf.keys"
)

var ErrInvalidECVRFKey = errors.New("invalid ecvrf key")

type ECVRFP256Adapter struct {
	keys        map[string]*ecdsa.PrivateKey
	auditReport string
	keySource   string
}

func init() {
	RegisterVRFAdapter(VRFAdapterECVRFP256Name, NewECVRFP256Adapter)
}

func NewECVRFP256Adapter(cfg config.VRFConfig) (VRFAdapter, error) {
	keys := make(map[string]*ecdsa.PrivateKey, len(cfg.Keys)*2)
	for publicKeyString, privateKeyBytes := range cfg.Keys {
		privateKey, err := ecvrfP256PrivateKey(privateKeyBytes)
		if err != nil {
			return nil, err
		}
		publicKey := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
		keys[string(publicKey)] = privateKey
		keys[base64.StdEncoding.EncodeToString(publicKey)] = privateKey
		if publicKeyString != "" {
			keys[publicKeyString] = privateKey
		}
	}
	auditReport := cfg.AuditReport
	if auditReport == "" {
		auditReport = ecvrfDependencyTag
	}
	keySource := cfg.KeySource
	if keySource == "" {
		keySource = ecvrfDefaultKeySourceName
	}
	return ECVRFP256Adapter{keys: keys, auditReport: auditReport, keySource: keySource}, nil
}

func (adapter ECVRFP256Adapter) Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	privateKey, found := adapter.keys[string(publicKey)]
	if !found {
		privateKey, found = adapter.keys[base64.StdEncoding.EncodeToString(publicKey)]
	}
	if !found || privateKey == nil {
		return nil, nil, ErrInvalidVRFKey
	}
	expectedPublicKey := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
	if !hmac.Equal(expectedPublicKey, publicKey) {
		return nil, nil, ErrInvalidVRFKey
	}
	output, proof, err = ecvrf.P256Sha256Tai.Prove(privateKey, ecvrfSeed(seed))
	if err != nil {
		return nil, nil, err
	}
	return output, proof, nil
}

func (adapter ECVRFP256Adapter) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	parsedPublicKey, err := ecvrfP256PublicKey(publicKey)
	if err != nil {
		return false
	}
	verifiedOutput, err := ecvrf.P256Sha256Tai.Verify(parsedPublicKey, ecvrfSeed(seed), proof)
	if err != nil {
		return false
	}
	return hmac.Equal(verifiedOutput, output)
}

func (adapter ECVRFP256Adapter) Metadata() VRFAdapterMetadata {
	return VRFAdapterMetadata{
		Name:                 VRFAdapterECVRFP256Name,
		Version:              "v1",
		Audited:              true,
		AuditReport:          adapter.auditReport,
		KeySource:            adapter.keySource,
		DomainSeparation:     true,
		ProofVerification:    true,
		DeterministicOutput:  true,
		MalformedInputFuzzed: true,
	}
}

func ECVRFP256PublicKeyFromPrivateKey(privateKeyBytes []byte) (types.PublicKey, error) {
	privateKey, err := ecvrfP256PrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	return elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y), nil
}

func ecvrfP256PrivateKey(privateKeyBytes []byte) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(privateKeyBytes)
	if d.Sign() <= 0 || d.Cmp(curve.Params().N) >= 0 {
		return nil, ErrInvalidECVRFKey
	}
	x, y := curve.ScalarBaseMult(d.FillBytes(make([]byte, 32)))
	if x == nil || y == nil {
		return nil, ErrInvalidECVRFKey
	}
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y},
		D:         d,
	}, nil
}

func ecvrfP256PublicKey(publicKeyBytes []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil || y == nil {
		return nil, ErrInvalidECVRFKey
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

func ecvrfSeed(seed []byte) []byte {
	prefixed := make([]byte, 0, len(ecvrfDomain)+len(seed))
	prefixed = append(prefixed, ecvrfDomain...)
	return append(prefixed, seed...)
}
