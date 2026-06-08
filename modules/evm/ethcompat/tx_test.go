package ethcompat

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
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

func TestIntrinsicGasWithChainConfigRejectsInvalidConfig(t *testing.T) {
	if _, err := IntrinsicGasWithChainConfigJSON([]byte{0x00}, nil, false, 0, `{invalid`); err == nil {
		t.Fatal("expected invalid chain config JSON to fail")
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

func TestDecodeRawTransactionPreservesSetCodeTx(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	authKey, err := gethcrypto.HexToECDSA("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	auth, err := gethtypes.SignSetCode(authKey, gethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(7),
		Address: gethcommon.HexToAddress("0x000000000000000000000000000000000000CAFe"),
		Nonce:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	tx := gethtypes.NewTx(&gethtypes.SetCodeTx{
		ChainID:   uint256.NewInt(7),
		Nonce:     12,
		GasTipCap: uint256.NewInt(2),
		GasFeeCap: uint256.NewInt(20),
		Gas:       80_000,
		To:        to,
		Value:     uint256.NewInt(3),
		Data:      []byte{0x12, 0x34},
		AuthList:  []gethtypes.SetCodeAuthorization{auth},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(big.NewInt(7)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	rawHex := "0x" + hex.EncodeToString(raw)
	decoded, err := DecodeRawTransaction(rawHex, DecodeOptions{ChainID: 7, BaseFee: 11})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Type != gethtypes.SetCodeTxType || decoded.Hash != signed.Hash().Hex() || decoded.Raw != strings.ToLower(rawHex) {
		t.Fatalf("unexpected set-code decoded tx: %+v", decoded)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Tags[TagType] != "4" || canonical.Tags[TagRaw] != strings.ToLower(rawHex) || canonical.Tags[TagChainID] != "7" {
		t.Fatalf("expected set-code metadata to be preserved: %+v", canonical.Tags)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); err != nil {
		t.Fatalf("expected set-code canonical tx to validate: %v", err)
	}
}

func TestDecodeRawTransactionPreservesUint256Value(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	value := new(big.Int).Lsh(big.NewInt(1), 80)
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(7),
		Nonce:     9,
		GasTipCap: big.NewInt(2),
		GasFeeCap: big.NewInt(20),
		Gas:       50_000,
		To:        &to,
		Value:     value,
		Data:      []byte{0x12, 0x34},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(big.NewInt(7)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRawTransaction("0x"+hex.EncodeToString(raw), DecodeOptions{ChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Value != 0 || decoded.ValueBig == nil || decoded.ValueBig.Cmp(value) != 0 {
		t.Fatalf("expected big value to be preserved, got %+v", decoded)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Args[6] != value.String() || canonical.Tags[TagValue] != value.String() {
		t.Fatalf("expected canonical big value, got args=%v tags=%v", canonical.Args, canonical.Tags)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); err != nil {
		t.Fatalf("expected big-value canonical tx to validate: %v", err)
	}
}

func TestDecodeRawTransactionPreservesAccessList(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	accessAddress := gethcommon.HexToAddress("0x000000000000000000000000000000000000CAFe")
	slot := gethcommon.HexToHash("0x01")
	tx := gethtypes.NewTx(&gethtypes.AccessListTx{
		ChainID:  big.NewInt(7),
		Nonce:    8,
		GasPrice: big.NewInt(13),
		Gas:      50_000,
		To:       &to,
		Data:     []byte{0x12, 0x34},
		AccessList: gethtypes.AccessList{{
			Address:     accessAddress,
			StorageKeys: []gethcommon.Hash{slot},
		}},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(big.NewInt(7)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRawTransaction("0x"+hex.EncodeToString(raw), DecodeOptions{ChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.AccessList) != 1 || decoded.AccessList[0].Address != types.Address(accessAddress.Hex()) || len(decoded.AccessList[0].StorageKeys) != 1 || decoded.AccessList[0].StorageKeys[0] != slot.Hex() {
		t.Fatalf("unexpected decoded access list: %+v", decoded.AccessList)
	}
	encoded, found := vexoapp.TxTag(decoded.Tx, TagAccessList)
	if !found || encoded == "" {
		t.Fatalf("expected canonical access-list tag in %s", decoded.Tx)
	}
	roundTrip, err := DecodeAccessList(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip) != 1 || roundTrip[0].Address != decoded.AccessList[0].Address || roundTrip[0].StorageKeys[0] != slot.Hex() {
		t.Fatalf("unexpected round-tripped access list: %+v", roundTrip)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); err != nil {
		t.Fatalf("expected canonical access-list tx to validate: %v", err)
	}
}

func TestDecodeRawTransactionPreservesBlobMetadata(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	blobHash := gethcommon.HexToHash("0x010203")
	tx := gethtypes.NewTx(&gethtypes.BlobTx{
		ChainID:    uint256.NewInt(7),
		Nonce:      10,
		GasTipCap:  uint256.NewInt(2),
		GasFeeCap:  uint256.NewInt(20),
		Gas:        50_000,
		To:         to,
		Value:      uint256.NewInt(3),
		Data:       []byte{0x12, 0x34},
		BlobFeeCap: uint256.NewInt(9),
		BlobHashes: []gethcommon.Hash{blobHash},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(big.NewInt(7)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRawTransaction("0x"+hex.EncodeToString(raw), DecodeOptions{ChainID: 7, BaseFee: 11, BlobBaseFee: 5})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.BlobGas == 0 || decoded.BlobGasFeeCap != 9 || decoded.BlobFee != decoded.BlobGas*5 || len(decoded.BlobHashes) != 1 || decoded.BlobHashes[0] != blobHash.Hex() {
		t.Fatalf("unexpected decoded blob metadata: %+v", decoded)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Tags[TagBlobGas] == "" || canonical.Tags[TagBlobGasFeeCap] != "9" || canonical.Tags[TagBlobHashes] == "" {
		t.Fatalf("expected blob canonical tags: %+v", canonical.Tags)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); err != nil {
		t.Fatalf("expected blob canonical tx to validate: %v", err)
	}
	if _, err := DecodeRawTransaction("0x"+hex.EncodeToString(raw), DecodeOptions{ChainID: 7, BaseFee: 11, BlobBaseFee: 10}); !errors.Is(err, ErrBlobFeeCapTooLow) {
		t.Fatalf("expected low blob fee cap rejection, got %v", err)
	}
}

func TestDecodeRawTransactionRejectsInvalidFeeCaps(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	cases := []struct {
		name      string
		gasTipCap *big.Int
		gasFeeCap *big.Int
		baseFee   uint64
		err       error
	}{
		{name: "fee cap below base fee", gasTipCap: big.NewInt(1), gasFeeCap: big.NewInt(9), baseFee: 10, err: ErrFeeCapTooLow},
		{name: "tip cap above fee cap", gasTipCap: big.NewInt(11), gasFeeCap: big.NewInt(10), baseFee: 0, err: ErrTipCapAboveFeeCap},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
				ChainID:   big.NewInt(7),
				Nonce:     9,
				GasTipCap: testCase.gasTipCap,
				GasFeeCap: testCase.gasFeeCap,
				Gas:       50_000,
				To:        &to,
				Value:     big.NewInt(1),
			})
			signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(big.NewInt(7)), key)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := signed.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeRawTransaction("0x"+hex.EncodeToString(raw), DecodeOptions{ChainID: 7, BaseFee: testCase.baseFee})
			if !errors.Is(err, testCase.err) {
				t.Fatalf("expected %v, got %v", testCase.err, err)
			}
		})
	}
}

func TestDecodeRawTransactionRejectsUnprotectedLegacyByDefault(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTransaction(3, to, big.NewInt(1), 21_000, big.NewInt(13), []byte{0x01})
	signed, err := gethtypes.SignTx(tx, gethtypes.HomesteadSigner{}, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	rawHex := "0x" + hex.EncodeToString(raw)
	if _, err := DecodeRawTransaction(rawHex, DecodeOptions{ChainID: 7}); !errors.Is(err, ErrUnprotectedLegacyTx) {
		t.Fatalf("expected unprotected legacy tx rejection, got %v", err)
	}
	decoded, err := DecodeRawTransaction(rawHex, DecodeOptions{ChainID: 7, AllowUnprotectedLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ChainID != 7 || decoded.Type != gethtypes.LegacyTxType {
		t.Fatalf("unexpected allowed legacy decode: %+v", decoded)
	}
	if err := ValidateCanonicalTxWithOptions(decoded.Tx, DecodeOptions{ChainID: 7, AllowUnprotectedLegacy: true}); err != nil {
		t.Fatalf("expected allowed legacy canonical tx to validate: %v", err)
	}
	if err := ValidateCanonicalTx(decoded.Tx, 7); !errors.Is(err, ErrUnprotectedLegacyTx) {
		t.Fatalf("expected default canonical validation to reject legacy tx, got %v", err)
	}
}

func TestVerifyBlobSidecarVerifiesKZGProofAndHashes(t *testing.T) {
	var blob kzg4844.Blob
	blob[0] = 1
	blob[31] = 2
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		t.Fatal(err)
	}
	versionedHash := gethcommon.Hash(kzg4844.CalcBlobHashV1(sha256.New(), &commitment)).Hex()
	sidecar := &gethtypes.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{blob},
		Commitments: []kzg4844.Commitment{commitment},
		Proofs:      []kzg4844.Proof{proof},
	}
	if err := VerifyBlobSidecar(sidecar, []string{versionedHash}); err != nil {
		t.Fatalf("expected valid sidecar: %v", err)
	}
	bundle, err := BlobSidecarBundleFromGeth(sidecar, []string{versionedHash})
	if err != nil {
		t.Fatal(err)
	}
	encodedBundle, err := EncodeBlobSidecarBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decodedBundle, err := DecodeBlobSidecarBundle(encodedBundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(decodedBundle.BlobHashes) != 1 || decodedBundle.BlobHashes[0] != versionedHash {
		t.Fatalf("unexpected decoded sidecar bundle: %+v", decodedBundle)
	}
	if err := VerifyBlobSidecar(sidecar, []string{gethcommon.HexToHash("0x01").Hex()}); !errors.Is(err, ErrInvalidBlobSidecar) {
		t.Fatalf("expected hash mismatch to fail, got %v", err)
	}
	tampered := sidecar
	tampered.Blobs = append([]kzg4844.Blob(nil), sidecar.Blobs...)
	tampered.Blobs[0][32] = 9
	if err := VerifyBlobSidecar(tampered, []string{versionedHash}); !errors.Is(err, ErrInvalidBlobSidecar) {
		t.Fatalf("expected tampered blob to fail, got %v", err)
	}
}

func TestValidateCanonicalTxRejectsTamperingAndWrongChain(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7, BaseFee: 11})
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
	canonical, err = vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Tags["fee"] = "1"
	tampered, err = vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTx(tampered, 7); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("expected tampered fee to be rejected, got %v", err)
	}
	canonical.Tags["fee"] = "273000"
	canonical.Tags[TagGasPrice] = "1"
	tampered, err = vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTx(tampered, 7); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("expected tampered gas price to be rejected, got %v", err)
	}
}

func TestValidateCanonicalTxForExecutionUsesCurrentBaseFee(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7, BaseFee: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTxForExecution(decoded.Tx, DecodeOptions{ChainID: 7, BaseFee: 12}); err != nil {
		t.Fatalf("expected execution validation to accept repriced base fee: %v", err)
	}
	if err := ValidateCanonicalTxForExecution(decoded.Tx, DecodeOptions{ChainID: 7, BaseFee: 21}); !errors.Is(err, ErrFeeCapTooLow) {
		t.Fatalf("expected current base fee to be enforced, got %v", err)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Tags["fee"] = "1"
	repriced, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalTxForExecution(repriced, DecodeOptions{ChainID: 7, BaseFee: 12}); err != nil {
		t.Fatalf("execution validation should not trust stale wrapper fee metadata: %v", err)
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
