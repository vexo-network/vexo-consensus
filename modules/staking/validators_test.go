package staking

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
	vexovalidator "github.com/vexo-network/vexo-consensus/validator"
)

// Regression: a delegation used to carry the validator key and metadata, so any delegator could swap
// the consensus key of an existing validator. Keys now enter staking state only through the operator's
// create-validator transaction (or genesis) and delegations never write them.

func TestStakingDelegateCannotReplaceValidatorKeyOrMetadata(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 2})
	fund(t, storage, "attacker", 100)

	// The old six-part delegate form carrying an attacker key is no longer a staking transaction.
	old := stakingTx("attacker", "delegate", "attacker", "validator-1", "50", keyArg("attacker-key"))
	if result := deliver(module, storage, 2, old); result.Code == 0 {
		t.Fatalf("expected key-carrying delegate to fail, got %+v", result)
	}
	mustDeliver(t, module, storage, 2, stakingTx("attacker", "delegate", "attacker", "validator-1", "50"))
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || string(updates[0].PublicKey) != "key-validator-1" || updates[0].VotingPower != 60 {
		t.Fatalf("delegation must not change the registered key, got %+v", updates)
	}
	if result := deliver(module, storage, 2, stakingTx("attacker", "create-validator", "attacker", "validator-1", "1", keyArg("attacker-key"))); result.Code == 0 || !strings.Contains(result.Log, ErrValidatorExists.Error()) {
		t.Fatalf("expected re-registration to fail, got %+v", result)
	}
}

func TestStakingCreateValidatorRequiresSignedOperatorAndUniqueKey(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	fund(t, storage, "alice", 100)
	key := keyArg("shared-key")
	if result := deliver(module, storage, 1, stakingTx("mallory", "create-validator", "alice", "validator-1", "10", key)); result.Code != codeUnauthorized {
		t.Fatalf("expected unauthorized registration, got %+v", result)
	}
	if result := deliver(module, storage, 1, types.Tx("staking:create-validator:alice:validator-1:10:"+key)); result.Code != codeUnauthorized {
		t.Fatalf("expected unsigned registration to fail, got %+v", result)
	}
	mustDeliver(t, module, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "10", key))
	if result := deliver(module, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-2", "10", key)); result.Code == 0 || !strings.Contains(result.Log, ErrDuplicateConsensusKey.Error()) {
		t.Fatalf("expected duplicate consensus key to fail, got %+v", result)
	}
	if power := mustPower(t, storage, "validator-2"); power != 0 {
		t.Fatalf("a refused registration left power %d", power)
	}
	if result := deliver(module, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-3", "1000", keyArg("other"))); result.Code == 0 || !strings.Contains(result.Log, ErrInsufficientBalance.Error()) {
		t.Fatalf("expected unfunded registration to fail, got %+v", result)
	}
}

func TestStakingCreateValidatorValidatesConsensusKeyFormat(t *testing.T) {
	storage := newStakingStore(t)
	fund(t, storage, "alice", 100)
	ed := NewModuleWithPolicy(Policy{CryptoBackend: config.CryptoBackendEd25519})
	if result := deliver(ed, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "10", keyArg("not-a-curve-point"))); result.Code == 0 || !strings.Contains(result.Log, ErrInvalidConsensusKey.Error()) {
		t.Fatalf("expected malformed ed25519 key to fail, got %+v", result)
	}
	// The identity point has small order and must be refused.
	smallOrder := base64.RawStdEncoding.EncodeToString(append([]byte{1}, make([]byte, 31)...))
	if result := deliver(ed, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "10", smallOrder)); result.Code == 0 || !strings.Contains(result.Log, ErrInvalidConsensusKey.Error()) {
		t.Fatalf("expected small-order ed25519 key to fail, got %+v", result)
	}
	bls := NewModuleWithPolicy(Policy{CryptoBackend: config.CryptoBackendBLS})
	if result := deliver(bls, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "10", keyArg("bls-key-without-proof"))); result.Code == 0 || !strings.Contains(result.Log, ErrInvalidConsensusKey.Error()) {
		t.Fatalf("expected bls key without proof of possession to fail, got %+v", result)
	}
	if power := mustPower(t, storage, "validator-1"); power != 0 {
		t.Fatalf("a refused registration left power %d", power)
	}
}

func TestStakingCreateValidatorAppliesAdmissionPolicy(t *testing.T) {
	storage := newStakingStore(t)
	policy := vexovalidator.NewConfigurableAdmissionPolicy(vexovalidator.AdmissionConfig{Permissionless: true, MinStake: 50, MaxValidators: 1})
	module := NewModuleWithPolicy(Policy{Admission: policy})
	fund(t, storage, "alice", 100)
	fund(t, storage, "bob", 100)
	if result := deliver(module, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "10", keyArg("a"))); result.Code == 0 || !strings.Contains(result.Log, vexovalidator.ErrInsufficientStake.Error()) {
		t.Fatalf("expected minimum stake refusal in the transaction, got %+v", result)
	}
	mustDeliver(t, module, storage, 1, stakingTx("alice", "create-validator", "alice", "validator-1", "60", keyArg("a")))
	if result := deliver(module, storage, 1, stakingTx("bob", "create-validator", "bob", "validator-2", "60", keyArg("b"))); result.Code == 0 || !strings.Contains(result.Log, vexovalidator.ErrValidatorSetFull.Error()) {
		t.Fatalf("expected full validator set refusal in the transaction, got %+v", result)
	}
}

// Regression: InitGenesis seeded nothing, so the first delegation to a genesis validator replaced its
// power by the delegated amount (100 -> 1).
func TestStakingInitGenesisSeedsGenesisValidators(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	genesisValidators := []types.ValidatorUpdate{
		{ID: "validator-1", Address: "vexovaloper-1", PublicKey: types.PublicKey("genesis-key-1"), VotingPower: 100, Stake: 100, Metadata: map[string]string{genesisAccountMetadataKey: "alice", "node_id": "n1"}},
		{ID: "validator-2", Address: "vexovaloper-2", PublicKey: types.PublicKey("genesis-key-2"), VotingPower: 40, Stake: 40},
	}
	ctx := vexoapp.Context{Store: storage, GenesisValidators: genesisValidators}
	genesis := vexoapp.GenesisState{"staking:stake:bob:validator-1": []byte("30")}
	if err := module.InitGenesis(ctx, genesis); err != nil {
		t.Fatal(err)
	}
	if power := mustPower(t, storage, "validator-1"); power != 100 {
		t.Fatalf("expected genesis power 100, got %d", power)
	}
	if alice, bob := mustStake(t, storage, "alice", "validator-1"), mustStake(t, storage, "bob", "validator-1"); alice != 70 || bob != 30 {
		t.Fatalf("expected operator stake 70 and explicit stake 30, got alice=%d bob=%d", alice, bob)
	}
	if operator := mustStake(t, storage, "vexovaloper-2", "validator-2"); operator != 40 {
		t.Fatalf("expected the validator address to hold genesis stake without an account, got %d", operator)
	}
	fund(t, storage, "carol", 10)
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 1})
	mustDeliver(t, module, storage, 1, stakingTx("carol", "delegate", "carol", "validator-1", "1"))
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 101 || updates[0].ID != "validator-1" || string(updates[0].PublicKey) != "genesis-key-1" || updates[0].Metadata["node_id"] != "n1" {
		t.Fatalf("one delegation must add to the genesis power, got %+v", updates)
	}
	// Genesis operators control their validator through the account in the genesis metadata.
	mustDeliver(t, module, storage, 1, stakingTx("alice", "set-commission", "validator-1", "100"))
	if result := deliver(module, storage, 1, stakingTx("vexovaloper-1", "set-commission", "validator-1", "100")); result.Code != codeUnauthorized {
		t.Fatalf("expected the operator address without account metadata to be refused, got %+v", result)
	}
}

func TestStakingInitGenesisRejectsInconsistentValidators(t *testing.T) {
	for name, validators := range map[string][]types.ValidatorUpdate{
		"duplicate id":  {{ID: "v", PublicKey: types.PublicKey("a"), VotingPower: 1}, {ID: "v", PublicKey: types.PublicKey("b"), VotingPower: 1}},
		"duplicate key": {{ID: "v1", PublicKey: types.PublicKey("a"), VotingPower: 1}, {ID: "v2", PublicKey: types.PublicKey("a"), VotingPower: 1}},
		"no power":      {{ID: "v", PublicKey: types.PublicKey("a")}},
		"no key":        {{ID: "v", VotingPower: 1}},
		"slash in id":   {{ID: "v/1", PublicKey: types.PublicKey("a"), VotingPower: 1}},
	} {
		storage := newStakingStore(t)
		err := NewModule().InitGenesis(vexoapp.Context{Store: storage, GenesisValidators: validators}, nil)
		if err == nil {
			t.Fatalf("%s: expected genesis failure", name)
		}
	}
	storage := newStakingStore(t)
	validators := []types.ValidatorUpdate{{ID: "v", PublicKey: types.PublicKey("a"), VotingPower: 5}}
	for name, genesis := range map[string]vexoapp.GenesisState{
		"stake exceeds power": {"staking:stake:alice:v": []byte("6")},
		"unknown validator":   {"staking:stake:alice:other": []byte("1")},
		"malformed key":       {"staking:stake:bad": []byte("1")},
		"zero amount":         {"staking:stake:alice:v": []byte("0")},
	} {
		if err := NewModule().InitGenesis(vexoapp.Context{Store: storage, GenesisValidators: validators}, genesis); err == nil {
			t.Fatalf("%s: expected genesis failure", name)
		}
	}
	if err := NewModule().InitGenesis(vexoapp.Context{Store: storage}, vexoapp.GenesisState{"staking:stake:bad": []byte("1")}); !errors.Is(err, ErrInvalidStakeRecord) {
		t.Fatalf("expected invalid stake record, got %v", err)
	}
}

// Regression: a "/" in a delegator or validator id made stake/<delegator>/<validator> ambiguous, so
// stake delegated with such ids was invisible to the per-validator prefix scans of slashing and rewards.
func TestStakingRejectsSlashAndNULInIdentifiers(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	fund(t, storage, "alice/validator-1", 10)
	fund(t, storage, "alice", 10)
	for _, tx := range []types.Tx{
		stakingTx("alice/validator-1", "delegate", "alice/validator-1", "x", "5"),
		stakingTx("alice", "delegate", "alice", "validator-1/x", "5"),
		stakingTx("alice/validator-1", "create-validator", "alice/validator-1", "validator-9", "5", keyArg("k9")),
		stakingTx("alice", "create-validator", "alice", "validator/9", "5", keyArg("k9")),
		stakingTx("alice", "undelegate", "alice", "validator/1", "5"),
		stakingTx("alice/validator-1", "claim-rewards", "alice/validator-1", "validator-1"),
		stakingTx("alice", "delegate", "alice", "validator\x00-1", "5"),
	} {
		if result := deliver(module, storage, 2, tx); result.Code != codeBadArguments {
			t.Fatalf("expected %q to be rejected as a bad argument, got %+v", tx, result)
		}
	}
	if response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"stake", "alice/validator-1", "x"}}); response.Code == 0 {
		t.Fatalf("expected query with slash identifier to fail, got %+v", response)
	}
	if balance := mustBalance(t, storage, "alice/validator-1"); balance != 10 {
		t.Fatalf("a refused transaction moved funds, balance %d", balance)
	}
	if err := NewModule().InitGenesis(vexoapp.Context{Store: storage, GenesisValidators: []types.ValidatorUpdate{{ID: "ok", PublicKey: types.PublicKey("k"), VotingPower: 1}}}, vexoapp.GenesisState{"staking:stake:a/b:ok": []byte("1")}); err == nil {
		t.Fatal("expected genesis stake with a slash in the delegator to fail")
	}
	if _, err := Stake(context.Background(), storage, "a/b", "ok"); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected invalid identity, got %v", err)
	}
}
