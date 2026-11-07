package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	vexoconfig "github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/p2p"
)

type testTLSFiles struct {
	caPath, certPath, keyPath string
	pool                      *x509.CertPool
	clientCert                tls.Certificate
}

// writeTestTLSFiles writes a CA and a node certificate for 127.0.0.1 signed by it, and returns
// a client certificate from the same CA for dialing.
func writeTestTLSFiles(t *testing.T) testTLSFiles {
	t.Helper()
	dir := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "vexo-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(serial int64, name string) ([]byte, *ecdsa.PrivateKey) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: name},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		return der, key
	}
	nodeDER, nodeKey := issue(2, "node")
	clientDER, clientKey := issue(3, "client")
	encodeKey := func(key *ecdsa.PrivateKey) []byte {
		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	}
	files := testTLSFiles{
		caPath:   filepath.Join(dir, "ca.pem"),
		certPath: filepath.Join(dir, "node.pem"),
		keyPath:  filepath.Join(dir, "node.key"),
		pool:     x509.NewCertPool(),
	}
	files.pool.AddCert(caCert)
	for path, data := range map[string][]byte{
		files.caPath:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		files.certPath: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: nodeDER}),
		files.keyPath:  encodeKey(nodeKey),
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files.clientCert = tls.Certificate{Certificate: [][]byte{clientDER}, PrivateKey: clientKey}
	return files
}

func loadTestStartInputs(t *testing.T) startInputs {
	t.Helper()
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return inputs
}

// Regression: the p2p tls_* settings were parsed but the transport was always built with
// TLSConfig nil, so peer traffic was plaintext whatever the operator configured.
func TestBuildGRPCTransportServesConfiguredTLS(t *testing.T) {
	files := writeTestTLSFiles(t)
	inputs := loadTestStartInputs(t)
	wire, err := buildGRPCTransport(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		AddrBookPath:     filepath.Join(t.TempDir(), "addrbook.json"),
		P2PTLSCertPath:   files.certPath,
		P2PTLSKeyPath:    files.keyPath,
		P2PTLSCAPath:     files.caPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer wire.Stop(context.Background())
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", wire.Address(), &tls.Config{
		RootCAs:      files.pool,
		Certificates: []tls.Certificate{files.clientCert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
	})
	if err != nil {
		t.Fatalf("expected the p2p listener to complete a TLS handshake, got %v", err)
	}
	_ = conn.Close()
}

func TestBuildGRPCTransportFailsOnUnreadableTLSFiles(t *testing.T) {
	inputs := loadTestStartInputs(t)
	missing := filepath.Join(t.TempDir(), "missing.pem")
	_, err := buildGRPCTransport(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		AddrBookPath:     filepath.Join(t.TempDir(), "addrbook.json"),
		P2PTLSCertPath:   missing,
		P2PTLSKeyPath:    missing,
		P2PTLSCAPath:     missing,
	})
	if err == nil {
		t.Fatal("expected startup to fail when the configured p2p tls files cannot be read")
	}
}

// Regression: learned address-book entries were fed to the transport as configured peers after a
// restart, so an attacker who got many low-sorting IDs learned could push operator peers out
// under the MaxPeers cap.
func TestBuildGRPCTransportNeverPromotesLearnedPeers(t *testing.T) {
	inputs := loadTestStartInputs(t)
	bookPath := filepath.Join(t.TempDir(), "addrbook.json")
	book, err := p2p.OpenAddrBookWithPolicy(bookPath, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aaa-1", "aaa-2", "aaa-3"} {
		book.Add(p2p.PeerID(id), "127.0.0.1:1"+id[4:]+"000", "handshake", false)
	}
	if err := book.Save(); err != nil {
		t.Fatal(err)
	}
	wire, err := buildGRPCTransport(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		AddrBookPath:     bookPath,
		P2PMaxPeers:      1,
		P2PPeers:         map[p2p.PeerID]string{"zzz-operator": "127.0.0.1:26999"},
	})
	if err != nil {
		t.Fatal(err)
	}
	configured := wire.ConfiguredPeerIDs()
	if len(configured) != 1 || configured[0] != "zzz-operator" {
		t.Fatalf("expected only the operator peer to be configured, got %v", configured)
	}
}

func TestNetworkSafetyRequiresP2PTLSForPublicPeers(t *testing.T) {
	base := startRuntimeConfig{
		RequireNetworkSafety: true,
		P2PEnabled:           true,
		P2PListenAddress:     "0.0.0.0:26656",
		P2PNodeID:            "node",
		P2PNodeKeyPath:       "node.key",
	}
	if err := validateRuntimeNetworkSafety(base); !errors.Is(err, vexoconfig.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected public p2p without tls to be refused, got %v", err)
	}
	withTLS := base
	withTLS.P2PTLSCertPath, withTLS.P2PTLSKeyPath, withTLS.P2PTLSCAPath = "node.pem", "node.key", "ca.pem"
	if err := validateRuntimeNetworkSafety(withTLS); err != nil {
		t.Fatalf("expected public p2p with tls to pass, got %v", err)
	}
	private := base
	private.P2PListenAddress = "127.0.0.1:26656"
	if err := validateRuntimeNetworkSafety(private); err != nil {
		t.Fatalf("expected loopback p2p without tls to pass, got %v", err)
	}
}

func TestNetworkSafetyRefusesPlaintextAdminTokensOffLoopback(t *testing.T) {
	cfg := startRuntimeConfig{RequireNetworkSafety: true, RPCEnabled: true, RPCAddress: "0.0.0.0:26657", RPCAdminToken: "secret"}
	if err := validateRuntimeNetworkSafety(cfg); !errors.Is(err, vexoconfig.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected a plaintext admin token on a public listener to be refused, got %v", err)
	}
	cfg.RPCTLSCertPath, cfg.RPCTLSKeyPath = "rpc.pem", "rpc.key"
	if err := validateRuntimeNetworkSafety(cfg); err != nil {
		t.Fatalf("expected an admin token over rpc tls to pass, got %v", err)
	}
	loopback := startRuntimeConfig{RequireNetworkSafety: true, RPCEnabled: true, RPCAddress: "127.0.0.1:26657", RPCAdminToken: "secret"}
	if err := validateRuntimeNetworkSafety(loopback); err != nil {
		t.Fatalf("expected a loopback admin listener to pass, got %v", err)
	}
}

func TestLoadRPCTLSConfig(t *testing.T) {
	files := writeTestTLSFiles(t)
	if cfg, err := loadRPCTLSConfig(startRuntimeConfig{}); err != nil || cfg != nil {
		t.Fatalf("expected no rpc tls without configuration, got %v %v", cfg, err)
	}
	cfg, err := loadRPCTLSConfig(startRuntimeConfig{RPCTLSCertPath: files.certPath, RPCTLSKeyPath: files.keyPath})
	if err != nil || cfg == nil || len(cfg.Certificates) != 1 || cfg.ClientAuth != tls.NoClientCert {
		t.Fatalf("expected a server certificate, got %+v %v", cfg, err)
	}
	mutual, err := loadRPCTLSConfig(startRuntimeConfig{RPCTLSCertPath: files.certPath, RPCTLSKeyPath: files.keyPath, RPCTLSCAPath: files.caPath})
	if err != nil || mutual.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("expected mutual tls with a ca, got %+v %v", mutual, err)
	}
	if _, err := loadRPCTLSConfig(startRuntimeConfig{RPCTLSCertPath: files.certPath}); err == nil {
		t.Fatal("expected a certificate without a key to fail")
	}
	if _, err := loadRPCTLSConfig(startRuntimeConfig{RPCTLSCertPath: files.certPath, RPCTLSKeyPath: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected an unreadable key to fail")
	}
}
