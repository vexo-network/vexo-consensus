package node

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	goruntime "runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/modules"
	"github.com/vexo-network/vexo-consensus/types"
)

// The transfers of the runtime workload. A transfer moves one avxo and pays throughputTransferFee
// for the throughputTransferGas it may use. The bank charges 10 gas for a send and the base fee of
// the fee market starts at 1; the fee is ten times the gas limit, so it still covers the gas after
// the market raised the base fee several times, as it does for blocks that use more gas than its
// target.
const (
	throughputAccountBalance = "1000000000000000000"
	throughputTransferAmount = "1"
	throughputTransferGas    = "100"
	throughputTransferFee    = "1000"

	// defaultThroughputAccounts is the number of accounts, and so of transactions in flight, a run
	// signs for unless VEXO_ACCOUNTS says otherwise. Genesis writes every account into the store of
	// every validator, a cost of seconds per thousand accounts before the run starts.
	defaultThroughputAccounts = 10_000
)

// runtimeThroughputWorkload is a stream of signed transfers between funded accounts, executed by
// the application stack a production node runs: the modules, the ante handler that verifies every
// signature and sequences and prices every transaction against the fee market, the node's Pebble
// store and a mempool that logs to a write-ahead log.
//
// Every validator has a client of its own, and every client accounts of its own, so a client
// submits the transactions of an account in nonce order to one validator. The pool admits a nonce
// only behind the one before it, and a block orders its transactions by a salt, not by nonce: an
// account has at most one transaction in flight, and its client submits the next one once the
// validator it submits to committed the previous. The accounts therefore bound the backlog of a run.
type runtimeThroughputWorkload struct {
	clients   int
	perClient int
	genesis   map[string][]byte
	txs       []string
	indexes   map[string]int
	bytes     int
}

// newRuntimeThroughputWorkload funds accounts accounts, at most one per transaction, and signs
// total transfers over them for the validators clients. It signs them all at once: a run signs
// before its validators start, so the signatures stay outside the measured window.
func newRuntimeThroughputWorkload(t *testing.T, total int, clients int, accounts int) *runtimeThroughputWorkload {
	t.Helper()
	if accounts <= 0 {
		t.Fatalf("VEXO_ACCOUNTS=%d is not a positive count", accounts)
	}
	started := time.Now()
	perClient := (min(accounts, total) + clients - 1) / clients
	owners := make([]throughputAccount, clients*perClient)
	if err := forEachParallel(len(owners), func(index int) error {
		owner, err := newThroughputAccount(index)
		owners[index] = owner
		return err
	}); err != nil {
		t.Fatal(err)
	}
	genesis := make(map[string][]byte, len(owners))
	for _, owner := range owners {
		genesis["bank:"+string(owner.address)] = []byte(throughputAccountBalance)
	}
	txs := make([]string, total)
	if err := forEachParallel(total, func(index int) error {
		// Transaction index is the position-th of its client; the client cycles through its
		// accounts, so an account signs every perClient-th of the client's transactions.
		client, position := index%clients, index/clients
		sender := client*perClient + position%perClient
		from, to := owners[sender], owners[(sender+1)%len(owners)]
		tx, err := signThroughputTransfer(from, to.address, position/perClient+1)
		txs[index] = string(tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	indexes := make(map[string]int, total)
	size := 0
	for index, tx := range txs {
		indexes[tx] = index
		size += len(tx)
	}
	if len(indexes) != total {
		t.Fatalf("the workload signed %d distinct transactions of %d", len(indexes), total)
	}
	t.Logf("signed %d transfers of %.0f bytes from %d accounts (%d per validator) in %s", total, float64(size)/float64(total), len(owners), perClient, time.Since(started))
	return &runtimeThroughputWorkload{clients: clients, perClient: perClient, genesis: genesis, txs: txs, indexes: indexes, bytes: size}
}

func (workload *runtimeThroughputWorkload) name() string {
	return "runtime"
}

func (workload *runtimeThroughputWorkload) appState() map[string][]byte {
	return workload.genesis
}

func (workload *runtimeThroughputWorkload) configure(cfg *Config) {
	cfg.Chain = productionChain(cfg.Chain, cfg.DataDir)
}

// newApplication builds the application as cmd/vexod does: modules.NewRuntimeWithChainConfig over
// the chain configuration of the node.
func (workload *runtimeThroughputWorkload) newApplication(t *testing.T, cfg Config) vexoapp.Application {
	t.Helper()
	application, err := modules.NewRuntimeWithChainConfig(cfg.Chain.ChainID, cfg.Chain)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func (workload *runtimeThroughputWorkload) tx(index int) types.Tx {
	return types.Tx(workload.txs[index])
}

// index looks a committed transaction up by its bytes, which a signed transaction keeps opaque.
func (workload *runtimeThroughputWorkload) index(t *testing.T, tx types.Tx) int {
	t.Helper()
	index, found := workload.indexes[string(tx)]
	if !found {
		t.Fatalf("not a throughput transaction: %q", tx)
	}
	return index
}

// predecessor returns the previous transaction of the sender of transaction index: the one the
// same client signed perClient transactions of its own earlier.
func (workload *runtimeThroughputWorkload) predecessor(index int) (int, bool) {
	before := index - workload.clients*workload.perClient
	return before, before >= 0
}

func (workload *runtimeThroughputWorkload) txBytes() float64 {
	return float64(workload.bytes) / float64(len(workload.txs))
}

// lossless is true: the accounts bound the backlog far below the limits of a pool, so a pool that
// loses a transaction is a failure of the run, not a measurement of it.
func (workload *runtimeThroughputWorkload) lossless() bool {
	return true
}

// TestRuntimeThroughputWorkloadSequencesEachAccount checks how the runtime workload deals out its
// transfers: every account is funded in genesis, belongs to the one validator its transfers are
// submitted to, and signs them with the nonces 1, 2, ... in the order that validator's client
// submits them, and the transfer a client waits for before it submits the next one of an account is
// the previous one of that account.
func TestRuntimeThroughputWorkloadSequencesEachAccount(t *testing.T) {
	const total, clients, accounts = 50, 4, 8
	workload := newRuntimeThroughputWorkload(t, total, clients, accounts)
	type transfer struct {
		sender, recipient string
		nonce             uint64
	}
	parse := func(index int) transfer {
		t.Helper()
		canonical, err := vexoapp.ParseCanonicalTx(workload.tx(index))
		if err != nil {
			t.Fatal(err)
		}
		nonce, err := strconv.ParseUint(canonical.Tags["nonce"], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return transfer{sender: canonical.Tags["signer"], recipient: canonical.Args[1], nonce: nonce}
	}
	if funded := len(workload.appState()); funded != accounts {
		t.Fatalf("genesis funds %d accounts, want %d", funded, accounts)
	}
	validators := map[string]int{}
	sequences := map[string]uint64{}
	for client := range clients {
		for index := client; index < total; index += clients {
			if found := workload.index(t, workload.tx(index)); found != index {
				t.Fatalf("transfer %d is looked up as transfer %d", index, found)
			}
			transfer := parse(index)
			for _, participant := range []string{transfer.sender, transfer.recipient} {
				if _, funded := workload.appState()["bank:"+participant]; !funded {
					t.Fatalf("transfer %d involves %s, which genesis does not fund", index, participant)
				}
			}
			if validator, known := validators[transfer.sender]; known && validator != client {
				t.Fatalf("%s signs for validator %d and %d", transfer.sender, validator, client)
			}
			validators[transfer.sender] = client
			if transfer.nonce != sequences[transfer.sender]+1 {
				t.Fatalf("transfer %d of %s carries nonce %d after nonce %d", index, transfer.sender, transfer.nonce, sequences[transfer.sender])
			}
			sequences[transfer.sender] = transfer.nonce
			before, gated := workload.predecessor(index)
			if gated != (transfer.nonce > 1) {
				t.Fatalf("transfer %d with nonce %d: gated %t", index, transfer.nonce, gated)
			}
			if !gated {
				continue
			}
			if previous := parse(before); previous.sender != transfer.sender || previous.nonce+1 != transfer.nonce {
				t.Fatalf("transfer %d waits for transfer %d, which is nonce %d of %s, not nonce %d of %s", index, before, previous.nonce, previous.sender, transfer.nonce-1, transfer.sender)
			}
		}
	}
	if len(validators) != accounts {
		t.Fatalf("%d accounts signed transfers, want %d", len(validators), accounts)
	}
}

// throughputAccount is a funded account of the runtime workload.
type throughputAccount struct {
	signer  vexocrypto.Ed25519Signer
	address types.Address
}

// newThroughputAccount derives the index-th account of the workload from its index, so every run
// signs the same transactions.
func newThroughputAccount(index int) (throughputAccount, error) {
	seed := sha256.Sum256([]byte("vexo-throughput-account-" + strconv.Itoa(index)))
	private := ed25519.NewKeyFromSeed(seed[:])
	signer, err := vexocrypto.NewEd25519Signer(private, private.Public().(ed25519.PublicKey))
	if err != nil {
		return throughputAccount{}, err
	}
	owner, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		return throughputAccount{}, err
	}
	return throughputAccount{signer: signer, address: owner}, nil
}

// signThroughputTransfer returns the signed transfer of one avxo from sender to recipient that
// carries nonce, built and signed as a client of the node does.
func signThroughputTransfer(sender throughputAccount, recipient types.Address, nonce int) (types.Tx, error) {
	payload, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "bank",
		Action: "send",
		Args:   []string{string(sender.address), string(recipient), throughputTransferAmount},
		Tags: map[string]string{
			"fee":    throughputTransferFee,
			"gas":    throughputTransferGas,
			"signer": string(sender.address),
			"nonce":  strconv.Itoa(nonce),
		},
	})
	if err != nil {
		return nil, err
	}
	return vexoapp.SignTx(throughputChainID, payload, sender.signer)
}

// productionChain returns chain with the application settings of a production node: the
// execution, bank, governance and mempool settings of the network-safe template, which require
// signed, sequenced, priced transactions and give the mempool a write-ahead log in dataDir, and the
// dynamic fee market that vexod init enables on top of it. What the validators sign their consensus
// messages with stays as chain has it, for VEXO_CRYPTO decides it.
func productionChain(chain config.Config, dataDir string) config.Config {
	production := config.NetworkSafeTemplate(chain.ChainID, dataDir)
	chain.Execution = production.Execution
	chain.Execution.DynamicBaseFee = true
	chain.Execution.MinBaseFee = 1
	chain.Execution.DynamicBlobBaseFee = true
	chain.Bank = production.Bank
	chain.Governance = production.Governance
	chain.Mempool = production.Mempool
	return chain
}

// forEachParallel calls work for every index of [0, count), on one goroutine per CPU for a
// contiguous range of indexes each, and returns the errors work returned.
func forEachParallel(count int, work func(index int) error) error {
	workers := max(1, min(goruntime.GOMAXPROCS(0), count))
	errs := make([]error, workers)
	var group sync.WaitGroup
	for worker := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := worker * count / workers; index < (worker+1)*count/workers; index++ {
				if errs[worker] = work(index); errs[worker] != nil {
					errs[worker] = fmt.Errorf("index %d: %w", index, errs[worker])
					return
				}
			}
		}()
	}
	group.Wait()
	return errors.Join(errs...)
}
