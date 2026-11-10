package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/lanes"
	"github.com/vexo-network/vexo-consensus/mempool"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// Lane dissemination.
//
// A validator batches the transactions submitted to it on its own lane and disseminates the
// batches to every validator, which store and acknowledge them (package lanes). A leader builds a
// block from the batches of all lanes and broadcasts it by naming them (consensus.LaneProposal):
// the leader no longer uploads the block's transactions, and the dissemination load is spread over
// all validators. Transactions a full node gossips are batched by the validator whose lane owns
// their sender (lanes.Owner); the others hold them and batch them themselves when the owner did
// not within lanes.FallbackDelay.

// newLanes returns the lane service of this validator. Its callbacks take no node lock: the
// service runs while the node starts and stops.
func (node *Node) newLanes(runtime *vexoruntime.Runtime, machine *consensus.StateMachine) *lanes.Lanes {
	signer := node.signer
	chainID := node.cfg.Chain.ChainID
	return lanes.New(lanes.Config{
		ChainID: chainID,
		Self:    node.cfg.ValidatorID,
		Sign: func(signType vexocrypto.SignType, domain vexocrypto.Domain, message []byte) (types.Signature, error) {
			height := machine.Status(context.Background()).Height
			if height == 0 {
				height = 1
			}
			return signWithConsensusPolicy(signer, vexocrypto.SignPolicy{ChainID: chainID, Height: height, Type: signType, Domain: domain}, message)
		},
		Signatures: runtime.Crypto.ConsensusVerifier,
		Aggregator: runtime.Crypto.ConsensusAggregator,
		Aggregates: runtime.Crypto.FinalityVerifier,
		ValidatorSet: func(context.Context) (validator.Set, error) {
			return machine.ProposalValidatorSet()
		},
		Pending: runtime.Mempool.Has,
		Ready:   func() { node.wakeConsensus(context.Background()) },
		Stored:  node.admitLaneBatch,
		Admit:   node.admitPeerMessage,
		Observe: node.observePeerMessage,
	})
}

// admitLaneBatch admits the transactions of another validator's batch into the pool, which so
// knows what is pending, refuses a duplicate submission, and keeps a transaction whose batch no
// block took up. The pool's verdicts do not reach the batch: blocks judge the transactions.
func (node *Node) admitLaneBatch(batch lanes.Batch) {
	runtime, err := node.Runtime()
	if err != nil || runtime.Mempool == nil {
		return
	}
	for _, tx := range batch.Txs {
		if isUnsignedEvidenceTx(tx) {
			continue
		}
		_ = node.admitMempoolTx(context.Background(), runtime.App, runtime.Mempool, tx)
	}
}

func (node *Node) laneService() *lanes.Lanes {
	node.mu.Lock()
	defer node.mu.Unlock()
	return node.lanes
}

// laneOwner reports whether this validator's lane owns tx: the lane of its sender, or of its hash
// when it names none.
func (node *Node) laneOwner(tx types.Tx) bool {
	machine, err := node.Consensus()
	if err != nil {
		return true
	}
	set, err := machine.ProposalValidatorSet()
	if err != nil {
		return true
	}
	key := []byte(tx)
	if slot, ok := slotOf(tx); ok {
		key = []byte(slot.signer)
	}
	return lanes.Owner(lanes.Members(set), key) == node.cfg.ValidatorID
}

// routeGossipedTx hands a transaction a peer gossiped to the lanes: the owner batches it, the
// others hold it in case the owner does not.
func (node *Node) routeGossipedTx(service *lanes.Lanes, tx types.Tx) {
	if node.laneOwner(tx) {
		service.Submit(tx)
		return
	}
	service.Hold(tx)
}

// laneCandidates returns the transactions of the batches the next block may carry, up to maxBytes,
// without those of the uncommitted ancestors (proposed) or repeated ones.
func laneCandidates(service *lanes.Lanes, maxBytes int64, proposed map[types.Hash]struct{}) []types.Tx {
	limit := int(min(maxBytes, int64(^uint(0)>>1)))
	seen := make(map[types.Hash]struct{})
	var txs []types.Tx
	for _, batch := range service.Candidates(limit) {
		for _, tx := range batch.Txs {
			hash := mempool.HashTx(tx)
			if _, inFlight := proposed[hash]; inFlight {
				continue
			}
			if _, repeated := seen[hash]; repeated {
				continue
			}
			seen[hash] = struct{}{}
			txs = append(txs, tx)
		}
	}
	return txs
}
