package app

import (
	"crypto/sha256"
	"encoding/binary"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

// VerifyCacheCapacity is the number of successful verifications a cache of the node's pipeline
// remembers. A transaction is verified by CheckTx, again when it is proposed, again when the block
// that carries it is validated and once more when that block executes, and between the first and
// the last it waits in the mempool, which holds up to 100000 transactions by default. The capacity
// leaves room for those and a few blocks beside them; what a full cache forgets is only verified
// again.
const VerifyCacheCapacity = 1 << 17

// VerifyDigest names one verification: the digest of everything its verdict is a function of.
type VerifyDigest [sha256.Size]byte

// NewVerifyDigest digests the kind of a verification, the inputs of its verdict other than the
// transaction (scope) and the bytes of tx. The kind and the scope carry their lengths, so no two
// verifications share a digest unless every one of their parts is equal. The digest holds no
// reference to its arguments.
func NewVerifyDigest(kind string, scope []byte, tx types.Tx) VerifyDigest {
	frame := make([]byte, 0, 2*binary.MaxVarintLen64+len(kind)+len(scope))
	frame = binary.AppendUvarint(frame, uint64(len(kind)))
	frame = append(frame, kind...)
	frame = binary.AppendUvarint(frame, uint64(len(scope)))
	frame = append(frame, scope...)
	hasher := sha256.New()
	hasher.Write(frame)
	hasher.Write(tx)
	var digest VerifyDigest
	hasher.Sum(digest[:0])
	return digest
}

// VerifyCache is a goroutine-safe set of verifications that succeeded. It holds only digests,
// never the transactions, and at most its capacity of them: an insertion into a full cache evicts
// the digest inserted longest ago, so the cache stays bounded whatever flows through it.
//
// A digest is present only if the verification it names succeeded, and a verification is a pure
// function of what the digest commits to, so a hit is exactly as good as verifying again. A failure
// is never remembered: a rejected transaction is rejected afresh every time, and whether a
// verification rejects may depend on the context it runs in. Nothing a verdict depends on may be
// left out of the digest, and a verification that reads state must not be remembered at all.
//
// A nil *VerifyCache remembers nothing.
type VerifyCache struct {
	mu       sync.Mutex
	capacity int
	proven   map[VerifyDigest]struct{}
	// order holds the digests in insertion order. Once it is full, order[oldest] is the digest
	// the next insertion replaces.
	order  []VerifyDigest
	oldest int
}

// NewVerifyCache returns an empty cache of at most capacity verifications, at least one.
func NewVerifyCache(capacity int) *VerifyCache {
	return &VerifyCache{capacity: max(capacity, 1), proven: make(map[VerifyDigest]struct{})}
}

// Contains reports whether the verification named by digest succeeded before.
func (cache *VerifyCache) Contains(digest VerifyDigest) bool {
	if cache == nil {
		return false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	_, found := cache.proven[digest]
	return found
}

// Add records that the verification named by digest succeeded.
func (cache *VerifyCache) Add(digest VerifyDigest) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, found := cache.proven[digest]; found {
		return
	}
	if len(cache.order) < cache.capacity {
		cache.order = append(cache.order, digest)
	} else {
		delete(cache.proven, cache.order[cache.oldest])
		cache.order[cache.oldest] = digest
		cache.oldest = (cache.oldest + 1) % cache.capacity
	}
	cache.proven[digest] = struct{}{}
}

// Len returns the number of verifications the cache remembers.
func (cache *VerifyCache) Len() int {
	if cache == nil {
		return 0
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return len(cache.proven)
}

// The kinds of verification the ante handler remembers.
const (
	verifyKindSigned        = "app/signed-tx"
	verifyKindAuthenticator = "app/authenticator"
)

// verifyScope is the scope of an ante verification: the chain id of the context, which every
// verification may depend on, then the inputs an authenticator names. The chain id carries its
// length, so the inputs that follow it can not be confused with a longer chain id.
func verifyScope(chainID string, inputs []byte) []byte {
	scope := make([]byte, 0, binary.MaxVarintLen64+len(chainID)+len(inputs))
	scope = binary.AppendUvarint(scope, uint64(len(chainID)))
	scope = append(scope, chainID...)
	return append(scope, inputs...)
}

// verifySigned verifies the signed envelope of tx for chainID. The verdict is a pure function of
// the chain id, the bytes of tx and the keeper's verifier, so a transaction that verified once is
// not verified again.
func (keeper *AnteKeeper) verifySigned(chainID string, tx types.Tx) error {
	digest := NewVerifyDigest(verifyKindSigned, verifyScope(chainID, nil), tx)
	if keeper.verified.Contains(digest) {
		return nil
	}
	if err := VerifySignedTx(chainID, tx, keeper.signatureVerifier()); err != nil {
		return err
	}
	keeper.verified.Add(digest)
	return nil
}

// authenticationDigest returns the digest under which a success of authenticator on tx is
// remembered. ok is false for an authenticator that does not name the inputs of its verdict, and
// for a call it declines to name them for: those run every time.
func authenticationDigest(ctx Context, authenticator TxAuthenticator, tx types.Tx) (digest VerifyDigest, ok bool) {
	cacheable, ok := authenticator.(CacheableAuthenticator)
	if !ok {
		return VerifyDigest{}, false
	}
	inputs, ok := cacheable.AuthenticationKey(ctx, tx)
	if !ok {
		return VerifyDigest{}, false
	}
	return NewVerifyDigest(verifyKindAuthenticator, verifyScope(ctx.ChainID, inputs), tx), true
}

// authenticateWith runs authenticator on tx, unless it is a CacheableAuthenticator that already
// succeeded on the same chain id, inputs and transaction bytes.
func (keeper *AnteKeeper) authenticateWith(ctx Context, authenticator TxAuthenticator, tx types.Tx) error {
	digest, remembered := authenticationDigest(ctx, authenticator, tx)
	if remembered && keeper.verified.Contains(digest) {
		return nil
	}
	if err := authenticator.AuthenticateTx(ctx, tx); err != nil {
		return err
	}
	if remembered {
		keeper.verified.Add(digest)
	}
	return nil
}

// preverify authenticates txs on up to GOMAXPROCS goroutines and returns when all of them are
// done. It only fills the cache. The admission of the block still runs one transaction after the
// other and finds the proofs there, so the verdicts it reaches, and the order it reaches them in,
// are those it reaches without this pass: every failure here is left to it. The pass is skipped
// when there is nothing to parallelize or nowhere to remember a proof, and it stops early when the
// context ends.
//
// Only authentications that are pure and safe for concurrent use run here: the signature of a
// signed envelope, and the Ethereum authenticator when it is a CacheableAuthenticator. An evidence
// authenticator reads state, and any other Ethereum authenticator may be anything, so those remain
// in the sequential pass. The pass hands the authenticators a context without the state store,
// which a CacheableAuthenticator must not read.
func (keeper *AnteKeeper) preverify(ctx Context, txs []types.Tx) {
	if keeper == nil || keeper.verified == nil {
		return
	}
	workers := min(runtime.GOMAXPROCS(0), len(txs))
	if workers < 2 {
		return
	}
	ctx.Store = nil
	goCtx := ctx.GoContext()
	var next atomic.Int64
	work := func() {
		for goCtx.Err() == nil {
			index := int(next.Add(1)) - 1
			if index >= len(txs) {
				return
			}
			keeper.preauthenticate(ctx, txs[index])
		}
	}
	var group sync.WaitGroup
	for range workers - 1 {
		group.Go(work)
	}
	work()
	group.Wait()
}

// preauthenticate does for tx what admission does to authenticate it, in the cases preverify may
// run beside the authentication of other transactions, and discards the verdict. It follows the
// branches of admitWith, so it authenticates exactly the transactions admission will.
func (keeper *AnteKeeper) preauthenticate(ctx Context, tx types.Tx) {
	// A panic of an authenticator on a hostile transaction must not end the node from a goroutine
	// its caller does not own. The sequential pass authenticates the transaction too, in the
	// caller's own goroutine, and meets the panic exactly as it would have without this pass.
	defer func() { _ = recover() }()
	adm, err := parseAdmission(tx)
	if err != nil {
		return
	}
	signed := txcodec.IsSigned(tx)
	switch {
	case txcodec.HasEthereumTag(adm.meta.Tags):
		authenticator, err := keeper.ethereumAuthenticator(adm, signed)
		if err != nil {
			return
		}
		if _, remembered := authenticationDigest(ctx, authenticator, tx); remembered {
			_ = keeper.authenticateWith(ctx, authenticator, tx)
		}
	case signed:
		_ = keeper.verifySigned(ctx.ChainID, tx)
	}
}
