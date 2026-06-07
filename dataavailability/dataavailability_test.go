package dataavailability

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestBuildProofComputesCommitmentMetadata(t *testing.T) {
	txs := []types.Tx{[]byte("one"), []byte("two")}
	proof := BuildProof(txs)
	if proof.Commitment == (types.Hash{}) {
		t.Fatal("expected non-zero commitment")
	}
	if proof.TxCount != 2 || proof.TotalBytes != 6 {
		t.Fatalf("unexpected proof: %+v", proof)
	}
	if proof.EncodedSize == 0 || proof.ChunkSize != DefaultChunkSize || proof.ChunkCount == 0 || proof.DataShards != DefaultDataShards || proof.ParityShards != DefaultParityShards || proof.ParityCount == 0 {
		t.Fatalf("expected chunk metadata in proof: %+v", proof)
	}
}

func TestVerifyAcceptsMatchingCommitment(t *testing.T) {
	block := AttachCommitment(types.Block{Txs: []types.Tx{[]byte("tx")}})
	if err := Verify(block.Header, block.Txs); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsMismatchedCommitment(t *testing.T) {
	block := AttachCommitment(types.Block{Txs: []types.Tx{[]byte("tx")}})
	err := Verify(block.Header, []types.Tx{[]byte("other")})
	if !errors.Is(err, ErrCommitmentMismatch) {
		t.Fatalf("expected commitment mismatch, got %v", err)
	}
}

func TestVerifyRejectsMissingCommitmentWithData(t *testing.T) {
	err := Verify(types.Header{}, []types.Tx{[]byte("tx")})
	if !errors.Is(err, ErrMissingData) {
		t.Fatalf("expected missing data commitment, got %v", err)
	}
}

func TestVerifyAllowsEmptyDataWithoutCommitment(t *testing.T) {
	if err := Verify(types.Header{}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAndVerifyChunkProof(t *testing.T) {
	txs := []types.Tx{[]byte("first transaction"), []byte("second transaction")}
	proof, err := BuildChunkProof(txs, 16, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChunkProof(proof); err != nil {
		t.Fatal(err)
	}
	proof.Data[0] ^= 0xff
	if err := VerifyChunkProof(proof); !errors.Is(err, ErrInvalidChunkProof) {
		t.Fatalf("expected invalid tampered proof, got %v", err)
	}
}

func TestRecoverTransactionsWithParityChunk(t *testing.T) {
	large := make([]byte, DefaultChunkSize*2+137)
	for index := range large {
		large[index] = byte(index % 251)
	}
	txs := []types.Tx{large, []byte("tail")}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunks(txs, proof.ChunkSize, proof.DataShards)
	if err != nil {
		t.Fatal(err)
	}
	available := make([]Chunk, 0, len(chunks)-1)
	for _, chunk := range chunks {
		if chunk.Index == 1 {
			continue
		}
		available = append(available, chunk)
	}
	recovered, err := RecoverTransactions(proof, available, parity)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != len(txs) || string(recovered[1]) != "tail" || string(recovered[0]) != string(large) {
		t.Fatalf("unexpected recovered txs: %d", len(recovered))
	}
}

func TestRecoverTransactionsWithReedSolomonParityChunks(t *testing.T) {
	large := make([]byte, DefaultChunkSize*4+32)
	for index := range large {
		large[index] = byte((index * 17) % 251)
	}
	txs := []types.Tx{large}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunks(txs, proof.ChunkSize, proof.DataShards)
	if err != nil {
		t.Fatal(err)
	}
	available := make([]Chunk, 0, len(chunks)-2)
	for _, chunk := range chunks {
		if chunk.Index == 1 || chunk.Index == 3 {
			continue
		}
		available = append(available, chunk)
	}
	recovered, err := RecoverTransactions(proof, available, parity)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || string(recovered[0]) != string(large) {
		t.Fatalf("unexpected recovered payload length=%d", len(recovered))
	}
}

func TestRecoverTransactionsWithAllTwoMissingChunkCombinations(t *testing.T) {
	large := make([]byte, DefaultChunkSize*4+32)
	for index := range large {
		large[index] = byte((index*31 + 7) % 251)
	}
	txs := []types.Tx{large}
	proof, err := BuildProofWithErasureOptions(txs, DefaultChunkSize, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunksWithOptions(txs, proof.ChunkSize, proof.DataShards, proof.ParityShards)
	if err != nil {
		t.Fatal(err)
	}
	for first := uint64(0); first < proof.DataShards; first++ {
		for second := first + 1; second < proof.DataShards; second++ {
			available := make([]Chunk, 0, len(chunks)-2)
			for _, chunk := range chunks {
				if chunk.Index == first || chunk.Index == second {
					continue
				}
				available = append(available, chunk)
			}
			recovered, err := RecoverTransactions(proof, available, parity)
			if err != nil {
				t.Fatalf("recover missing chunks %d/%d: %v", first, second, err)
			}
			if len(recovered) != 1 || string(recovered[0]) != string(large) {
				t.Fatalf("unexpected recovered payload for missing chunks %d/%d", first, second)
			}
		}
	}
}

func TestRecoverTransactionsRejectsTooManyMissingChunksInGroup(t *testing.T) {
	large := make([]byte, DefaultChunkSize*4+32)
	txs := []types.Tx{large}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunks(txs, proof.ChunkSize, proof.DataShards)
	if err != nil {
		t.Fatal(err)
	}
	available := make([]Chunk, 0, len(chunks)-3)
	for _, chunk := range chunks {
		if chunk.Index == 0 || chunk.Index == 1 || chunk.Index == 2 {
			continue
		}
		available = append(available, chunk)
	}
	if _, err := RecoverTransactions(proof, available, parity); !errors.Is(err, ErrTooManyMissing) {
		t.Fatalf("expected too many missing chunks, got %v", err)
	}
}

func TestPlanAndVerifySamples(t *testing.T) {
	payload := make([]byte, DefaultChunkSize*5+17)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	txs := []types.Tx{payload}
	proof, err := BuildProofWithErasureOptions(txs, DefaultChunkSize, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	request, err := PlanSamples("vexo-test", 12, proof, SamplePolicy{Samples: 3, MinSamples: 2}, []byte("round-qc"))
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Indices) != 3 || request.Seed == (types.Hash{}) {
		t.Fatalf("unexpected sample request: %+v", request)
	}
	proofs := make([]ChunkProof, 0, len(request.Indices))
	for _, index := range request.Indices {
		chunkProof, err := BuildChunkProof(txs, proof.ChunkSize, index)
		if err != nil {
			t.Fatal(err)
		}
		proofs = append(proofs, chunkProof)
	}
	report, err := VerifySamples(request, proofs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verified != 3 || report.Requested != 3 || report.CoverageBPS == 0 {
		t.Fatalf("unexpected sample report: %+v", report)
	}
}

func TestVerifySamplesRejectsMissingDuplicateAndUnrequestedProofs(t *testing.T) {
	txs := []types.Tx{[]byte("a"), []byte("b"), []byte("c")}
	proof, err := BuildProofWithOptions(txs, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	request, err := PlanSamples("vexo-test", 1, proof, SamplePolicy{Samples: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySamples(request, nil); !errors.Is(err, ErrInsufficientChunks) {
		t.Fatalf("expected missing sample rejection, got %v", err)
	}
	chunkProof, err := BuildChunkProof(txs, proof.ChunkSize, request.Indices[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySamples(request, []ChunkProof{chunkProof, chunkProof}); !errors.Is(err, ErrInvalidSampleSet) {
		t.Fatalf("expected duplicate sample rejection, got %v", err)
	}
	otherIndex := (request.Indices[0] + 1) % proof.ChunkCount
	if otherIndex == request.Indices[0] {
		t.Skip("single chunk proof cannot create an unrequested index")
	}
	otherProof, err := BuildChunkProof(txs, proof.ChunkSize, otherIndex)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySamples(request, []ChunkProof{otherProof}); !errors.Is(err, ErrInvalidSampleSet) {
		t.Fatalf("expected unrequested sample rejection, got %v", err)
	}
}

func TestRecoverTransactionsRejectsInvalidErasureMetadata(t *testing.T) {
	txs := []types.Tx{[]byte("metadata")}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunks(txs, proof.ChunkSize, proof.DataShards)
	if err != nil {
		t.Fatal(err)
	}
	proof.ParityCount++
	if _, err := RecoverTransactions(proof, chunks, parity); !errors.Is(err, ErrInvalidChunkProof) {
		t.Fatalf("expected invalid chunk proof, got %v", err)
	}
}

func TestRecoverTransactionsRejectsDuplicateChunks(t *testing.T) {
	txs := []types.Tx{[]byte("duplicate")}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	parity, err := BuildParityChunks(txs, proof.ChunkSize, proof.DataShards)
	if err != nil {
		t.Fatal(err)
	}
	duplicateData := append([]Chunk(nil), chunks...)
	duplicateData = append(duplicateData, chunks[0])
	if _, err := RecoverTransactions(proof, duplicateData, parity); !errors.Is(err, ErrInvalidChunkProof) {
		t.Fatalf("expected invalid duplicate data chunk, got %v", err)
	}
	duplicateParity := append([]Chunk(nil), parity...)
	duplicateParity = append(duplicateParity, parity[0])
	if _, err := RecoverTransactions(proof, chunks, duplicateParity); !errors.Is(err, ErrInvalidChunkProof) {
		t.Fatalf("expected invalid duplicate parity chunk, got %v", err)
	}
}

func TestRecoverTransactionsRejectsTamperedChunk(t *testing.T) {
	txs := []types.Tx{[]byte("a"), []byte("b")}
	proof := BuildProof(txs)
	chunks, err := BuildChunks(txs, proof.ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	chunks[0].Data[0] ^= 0xff
	if _, err := RecoverTransactions(proof, chunks, nil); !errors.Is(err, ErrCommitmentMismatch) {
		t.Fatalf("expected commitment mismatch, got %v", err)
	}
}
