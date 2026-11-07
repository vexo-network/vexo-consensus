package main

import (
	"errors"
	"testing"

	vexoconfig "github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestListenAddressClassification(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		private  bool
		loopback bool
	}{
		{name: "loopback ipv4", address: "127.0.0.1:26656", private: true, loopback: true},
		{name: "loopback ipv6", address: "[::1]:26656", private: true, loopback: true},
		{name: "localhost", address: "localhost:26656", private: true, loopback: true},
		{name: "bare loopback host", address: "127.0.0.1", private: true, loopback: true},
		{name: "bare bracketed loopback host", address: "[::1]", private: true, loopback: true},
		{name: "rfc1918 10/8", address: "10.1.2.3:26656", private: true},
		{name: "rfc1918 172.16/12", address: "172.16.0.1:26656", private: true},
		{name: "rfc1918 192.168/16", address: "192.168.1.1:26656", private: true},
		{name: "unique local ipv6", address: "[fd00::1]:26656", private: true},
		{name: "empty host binds every interface", address: ":26656"},
		{name: "empty bracketed host binds every interface", address: "[]:26656"},
		{name: "empty address", address: ""},
		{name: "unspecified ipv4", address: "0.0.0.0:26656"},
		{name: "unspecified ipv6", address: "[::]:26656"},
		{name: "bare unspecified ipv4", address: "0.0.0.0"},
		{name: "bare unspecified ipv6", address: "::"},
		{name: "ipv4-mapped unspecified", address: "[::ffff:0.0.0.0]:26656"},
		{name: "public ipv4", address: "203.0.113.7:26656"},
		{name: "public ipv6", address: "[2001:db8::1]:26656"},
		{name: "host name", address: "validator-1.example.com:26656"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isPrivateListenAddress(testCase.address); got != testCase.private {
				t.Fatalf("isPrivateListenAddress(%q) = %v, want %v", testCase.address, got, testCase.private)
			}
			if got := isLoopbackListenAddress(testCase.address); got != testCase.loopback {
				t.Fatalf("isLoopbackListenAddress(%q) = %v, want %v", testCase.address, got, testCase.loopback)
			}
		})
	}
}

func TestAllInterfaceListenersAreTreatedAsPublic(t *testing.T) {
	for _, address := range []string{":26656", "0.0.0.0:26656", "[::]:26656"} {
		t.Run("p2p "+address, func(t *testing.T) {
			if !requiresAuthenticatedP2P(startRuntimeConfig{P2PListenAddress: address}) {
				t.Fatalf("a p2p listener on %q is reachable from outside and must require authentication", address)
			}
		})
		t.Run("rpc "+address, func(t *testing.T) {
			if !nodeHasPublicExposure(startRuntimeConfig{RPCEnabled: true, RPCAddress: address}) {
				t.Fatalf("an rpc listener on %q is reachable from outside and must count as public exposure", address)
			}
			err := validateRuntimeNetworkSafety(startRuntimeConfig{
				RequireNetworkSafety: true,
				RPCEnabled:           true,
				RPCAddress:           address,
				RPCEVMAccountKeyEnvs: []string{"EVM_KEY"},
			})
			if !errors.Is(err, vexoconfig.ErrUnsafeNetworkConfig) {
				t.Fatalf("expected managed evm keys on a %q rpc listener to be refused, got %v", address, err)
			}
		})
	}
	if requiresAuthenticatedP2P(startRuntimeConfig{
		P2PListenAddress: "127.0.0.1:26656",
		P2PPeers:         map[p2p.PeerID]string{"bob": "10.0.0.2:26656"},
	}) {
		t.Fatal("loopback and private-network peers must stay exempt from authenticated p2p")
	}
}
