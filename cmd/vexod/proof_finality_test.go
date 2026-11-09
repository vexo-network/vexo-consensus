package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRunProofDetectFinalityConflict(t *testing.T) {
	signers := make(map[types.ValidatorID]vexocrypto.Ed25519Signer)
	validators := make([]validator.Validator, 0, 4)
	for _, id := range []types.ValidatorID{"a", "b", "c", "d"} {
		signer, err := vexocrypto.GenerateEd25519Signer()
		if err != nil {
			t.Fatal(err)
		}
		signers[id] = signer
		validators = append(validators, validator.Validator{ID: id, VotingPower: 1, PublicKey: signer.PublicKey()})
	}
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		t.Fatal(err)
	}
	validatorSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	first := signedFinalityProof(t, validatorSet, signers, types.Hash{1}, []types.ValidatorID{"a", "b", "c"})
	second := signedFinalityProof(t, validatorSet, signers, types.Hash{2}, []types.ValidatorID{"a", "b", "d"})

	dir := t.TempDir()
	firstPath := writeJSON(t, filepath.Join(dir, "first.json"), first)
	secondPath := writeJSON(t, filepath.Join(dir, "second.json"), second)
	validatorsPath := writeJSON(t, filepath.Join(dir, "validators.json"), validators)

	var output bytes.Buffer
	err = runCommand(&output, &bytes.Buffer{}, []string{
		"proof",
		"detect-finality-conflict",
		"--first", firstPath,
		"--second", secondPath,
		"--validator-set", validatorsPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"finality conflict detected",
		"height: 1",
		"double_signers: [a b]",
		"double_sign_power: 2",
		"meets_fault_threshold: true",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in output:\n%s", expected, output.String())
		}
	}
}

// signedFinalityProof builds a finality proof under the 2-chain commit rule: the block at height 1
// with the given application hash, certified in view 0, and its child certified in view 1.
func signedFinalityProof(t *testing.T, validatorSet validator.Set, signers map[types.ValidatorID]vexocrypto.Ed25519Signer, appHash types.Hash, signerIDs []types.ValidatorID) finality.Proof {
	t.Helper()
	header := types.Header{
		ChainID:              "vexo-test",
		Height:               1,
		AppHash:              appHash,
		ConsensusHash:        appHash,
		ValidatorSetHeight:   finality.FirstValidatorSetHeight,
		ValidatorSetHash:     validatorSet.Hash(),
		NextValidatorSetHash: validatorSet.Hash(),
	}
	childHeader := types.Header{
		ChainID:              header.ChainID,
		Height:               header.Height + 1,
		Round:                1,
		JustifyRound:         0,
		PreviousBlockHash:    finality.HeaderHash(header),
		ValidatorSetHeight:   finality.ChildValidatorSetHeight(header),
		StateHeight:          finality.ChildStateHeight(header),
		ValidatorSetHash:     validatorSet.Hash(),
		NextValidatorSetHash: validatorSet.Hash(),
	}
	return finality.NewProof(header, signedQuorumCert(t, validatorSet, signers, header, 0, signerIDs)).
		WithChild(childHeader, signedQuorumCert(t, validatorSet, signers, childHeader, 1, signerIDs))
}

func signedQuorumCert(t *testing.T, validatorSet validator.Set, signers map[types.ValidatorID]vexocrypto.Ed25519Signer, header types.Header, round types.Round, signerIDs []types.ValidatorID) finality.QuorumCert {
	t.Helper()
	cert := finality.QuorumCert{
		Height:    header.Height,
		Round:     round,
		BlockHash: finality.HeaderHash(header),
		Signers:   finality.EncodeSigners(signerIDs),
	}
	signatures := make([]types.Signature, 0, len(signerIDs))
	for _, id := range signerIDs {
		signature, err := vexocrypto.SignWithDomain(signers[id], vexocrypto.DomainConsensusVote, cert.SignBytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures = append(signatures, signature)
		validatorInfo, found := validatorSet.Get(id)
		if !found {
			t.Fatalf("missing validator %s", id)
		}
		cert.VotingPower += validatorInfo.VotingPower
	}
	aggregate, err := vexocrypto.CombineEd25519Signatures(signatures)
	if err != nil {
		t.Fatal(err)
	}
	cert.Signature = aggregate
	return cert
}

func writeJSON(t *testing.T, path string, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
