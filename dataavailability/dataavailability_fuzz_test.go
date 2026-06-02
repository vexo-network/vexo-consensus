package dataavailability

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func FuzzVerify(f *testing.F) {
	f.Add([]byte("tx-a"), []byte("tx-b"), false, false)
	f.Add([]byte("tx-a"), []byte{}, true, false)
	f.Add([]byte{}, []byte{}, false, true)
	f.Fuzz(func(t *testing.T, first []byte, second []byte, mutate bool, omitCommitment bool) {
		txs := make([]types.Tx, 0, 2)
		if len(first) > 0 {
			txs = append(txs, append(types.Tx(nil), first...))
		}
		if len(second) > 0 {
			txs = append(txs, append(types.Tx(nil), second...))
		}

		header := types.Header{}
		if !omitCommitment {
			header.ConsensusHash = Commitment(txs)
			if mutate {
				header.ConsensusHash[0] ^= 0xff
			}
		}

		err := Verify(header, txs)
		switch {
		case omitCommitment && len(txs) > 0:
			if !errors.Is(err, ErrMissingData) {
				t.Fatalf("expected missing data error, got %v", err)
			}
		case mutate && !omitCommitment:
			if !errors.Is(err, ErrCommitmentMismatch) {
				t.Fatalf("expected commitment mismatch, got %v", err)
			}
		case err != nil:
			t.Fatalf("expected valid data availability proof, got %v", err)
		}
	})
}
