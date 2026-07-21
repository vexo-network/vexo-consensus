package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
)

type tlsStatusProvider struct{}

func (tlsStatusProvider) Status(ctx context.Context) node.Status {
	return node.Status{ChainID: "vexo-chain", EVMChainID: 2026, Running: true}
}

func TestStartRPCTLSServesWeb3ChainID(t *testing.T) {
	home := t.TempDir()
	caPEM, caCert, caKey, err := generateNetworkTLSCA("vexo-rpc-test")
	if err != nil {
		t.Fatal(err)
	}
	leafCertPEM, leafKeyPEM, _, err := generateNetworkTLSMaterial("vexo-rpc-test", networkTLSSubjectAltNames{}, caCert, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(home, "rpc.crt")
	keyPath := filepath.Join(home, "rpc.key")
	if err := os.WriteFile(certPath, leafCertPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, leafKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	tlsConfig, err := loadRPCTLSConfig(startRuntimeConfig{
		RPCTLSCertPath: certPath,
		RPCTLSKeyPath:  keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	serverErr := make(chan error, 1)
	addr, shutdown, err := startRPCServerWithConfig(tlsStatusProvider{}, "127.0.0.1:0", vexorpc.Config{
		TLSConfig: tlsConfig,
	}, serverErr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = shutdown(context.Background())
		select {
		case err := <-serverErr:
			if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("unexpected server error: %v", err)
			}
		default:
		}
	}()

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatal("expected CA PEM to parse")
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				ServerName: "127.0.0.1",
				MinVersion: tls.VersionTLS13,
			},
			DisableKeepAlives: true,
		},
	}
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`)
	var response *http.Response
	for i := 0; i < 50; i++ {
		req, err := http.NewRequest(http.MethodPost, "https://"+addr+"/web3", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://remix.ethereum.org")
		response, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if response == nil {
		t.Fatal("expected HTTPS response")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected status %d body=%s", response.StatusCode, string(payload))
	}
	var payload struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  string `json:"result"`
		Error   any    `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != nil || payload.Result != "0x7ea" {
		t.Fatalf("unexpected chain-id payload: %+v", payload)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("unexpected cors origin %q", got)
	}
}
