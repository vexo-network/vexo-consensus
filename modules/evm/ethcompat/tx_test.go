package ethcompat

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func TestDecodeRawTransactionBuildsSignedCallCanonicalTx(t *testing.T) {
	raw, hash := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7, BaseFee: 11})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Hash != hash || decoded.ContractCreation || decoded.To == nil {
		t.Fatalf("unexpected decoded tx: %+v", decoded)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Module != "evm" || canonical.Action != "call" || canonical.Tags[TagHash] != hash || canonical.Tags[TagRaw] != strings.ToLower(raw) {
		t.Fatalf("unexpected canonical tx: %+v", canonical)
	}
	if canonical.Tags["fee"] != "273000" || canonical.Tags[TagGasPrice] != "13" {
		t.Fatalf("expected base-fee aware effective gas price tags: %+v", canonical.Tags)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); err != nil {
		t.Fatalf("expected canonical tx to validate: %v", err)
	}
}

func TestDecodeRawTransactionBuildsContractCreationCanonicalTx(t *testing.T) {
	raw, hash := signedRawTestTx(t, 7, true)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7, BaseFee: 1})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Hash != hash || !decoded.ContractCreation || decoded.ContractAddress == "" {
		t.Fatalf("unexpected decoded contract creation: %+v", decoded)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Action != "eth_deploy" || canonical.Args[2] != "6000" {
		t.Fatalf("unexpected creation canonical tx: %+v", canonical)
	}
}

func TestValidateCanonicalTxRejectsTamperingAndWrongChain(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 8); err == nil {
		t.Fatal("expected wrong chain ID to be rejected")
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Args[2] = "0x0000000000000000000000000000000000000001"
	tampered, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTx(tampered, 7); err == nil {
		t.Fatal("expected tampered canonical tx to be rejected")
	}
}

func signedRawTestTx(t *testing.T, chainID uint64, create bool) (string, string) {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	var to *gethcommon.Address
	data := []byte{0x12, 0x34}
	if create {
		data = []byte{0x60, 0x00}
	} else {
		address := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
		to = &address
	}
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     7,
		GasTipCap: big.NewInt(2),
		GasFeeCap: big.NewInt(20),
		Gas:       21_000,
		To:        to,
		Value:     big.NewInt(3),
		Data:      data,
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex()
}
