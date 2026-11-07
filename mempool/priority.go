package mempool

import (
	"strconv"

	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

// The helpers below read transaction metadata through txcodec, the parser the
// application runtime uses, so the pool and the ante handler can never
// disagree about a signer, nonce or fee. A payload that does not parse has no
// metadata; the pool itself rejects such payloads at admission.

// TxFee returns the fee tag saturated to uint64.
func TxFee(tx types.Tx) uint64 {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil {
		return 0
	}
	return meta.FeeSaturated()
}

// TxPriority returns the priority tag.
func TxPriority(tx types.Tx) uint64 {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil {
		return 0
	}
	return meta.Priority
}

// TxNonce returns the nonce tag.
func TxNonce(tx types.Tx) (uint64, bool) {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil || !meta.HasNonce {
		return 0, false
	}
	return meta.Nonce, true
}

// TxSigner returns the normalized account identity named by the signer tag.
func TxSigner(tx types.Tx) (string, bool) {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil || meta.Signer == "" {
		return "", false
	}
	return txcodec.NormalizeAddress(meta.Signer), true
}

// ReplacementKey identifies the account sequence slot a transaction occupies.
func ReplacementKey(tx types.Tx) (string, bool) {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil {
		return "", false
	}
	return replacementKey(meta)
}

func replacementKey(meta txcodec.Meta) (string, bool) {
	if meta.Signer == "" || !meta.HasNonce {
		return "", false
	}
	return txcodec.NormalizeAddress(meta.Signer) + "/" + strconv.FormatUint(meta.Nonce, 10), true
}

func replacementPriceBumped(oldFee uint64, oldPriority uint64, newFee uint64, newPriority uint64, bumpBPS uint64) bool {
	if bumpBPS == 0 {
		bumpBPS = 1000
	}
	if oldFee == 0 && oldPriority == 0 {
		return newFee > 0 || newPriority > 0
	}
	return bumpedByBPS(newFee, oldFee, bumpBPS) || bumpedByBPS(newPriority, oldPriority, bumpBPS)
}

func bumpedByBPS(newValue uint64, oldValue uint64, bumpBPS uint64) bool {
	if oldValue == 0 {
		return newValue > 0
	}
	maxUint64 := ^uint64(0)
	if oldValue > maxUint64/(10000+bumpBPS) {
		return newValue == maxUint64
	}
	required := (oldValue*(10000+bumpBPS) + 9999) / 10000
	return newValue >= required
}
