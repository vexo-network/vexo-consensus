package dataavailability

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrCommitmentMismatch = errors.New("data availability commitment mismatch")
	ErrMissingData        = errors.New("data availability data is missing")
	ErrInvalidChunkSize   = errors.New("invalid data availability chunk size")
	ErrInvalidDataShards  = errors.New("invalid data availability data shards")
	ErrInvalidChunkProof  = errors.New("invalid data availability chunk proof")
	ErrInsufficientChunks = errors.New("insufficient data availability chunks")
	ErrTooManyMissing     = errors.New("too many missing data availability chunks")
	ErrInvalidEncoding    = errors.New("invalid data availability transaction encoding")
)

const (
	DefaultChunkSize  uint64 = 1024
	DefaultDataShards uint64 = 8
)

type Proof struct {
	Commitment  types.Hash
	TxCount     uint64
	TotalBytes  uint64
	EncodedSize uint64
	ChunkSize   uint64
	ChunkCount  uint64
	DataShards  uint64
	ParityCount uint64
}

type Chunk struct {
	Index uint64
	Data  []byte
}

type ChunkProof struct {
	Commitment types.Hash
	ChunkSize  uint64
	ChunkCount uint64
	Index      uint64
	Data       []byte
	Path       []MerkleSibling
}

type MerkleSibling struct {
	Hash types.Hash
	Left bool
}

func BuildProof(txs []types.Tx) Proof {
	proof, _ := BuildProofWithOptions(txs, DefaultChunkSize, DefaultDataShards)
	return proof
}

func BuildProofWithOptions(txs []types.Tx, chunkSize uint64, dataShards uint64) (Proof, error) {
	if dataShards == 0 {
		return Proof{}, ErrInvalidDataShards
	}
	var totalBytes uint64
	for _, tx := range txs {
		totalBytes += uint64(len(tx))
	}
	encoded := encodeTransactions(txs)
	chunks, err := splitChunks(encoded, chunkSize)
	if err != nil {
		return Proof{}, err
	}
	return Proof{
		Commitment:  chunkRoot(chunks),
		TxCount:     uint64(len(txs)),
		TotalBytes:  totalBytes,
		EncodedSize: uint64(len(encoded)),
		ChunkSize:   chunkSize,
		ChunkCount:  uint64(len(chunks)),
		DataShards:  dataShards,
		ParityCount: parityCount(uint64(len(chunks)), dataShards),
	}, nil
}

func Commitment(txs []types.Tx) types.Hash {
	proof := BuildProof(txs)
	return proof.Commitment
}

func BuildChunks(txs []types.Tx, chunkSize uint64) ([]Chunk, error) {
	return splitChunks(encodeTransactions(txs), chunkSize)
}

func BuildParityChunks(txs []types.Tx, chunkSize uint64, dataShards uint64) ([]Chunk, error) {
	if dataShards == 0 {
		return nil, ErrInvalidDataShards
	}
	chunks, err := BuildChunks(txs, chunkSize)
	if err != nil {
		return nil, err
	}
	return parityChunks(chunks, chunkSize, dataShards), nil
}

func BuildChunkProof(txs []types.Tx, chunkSize uint64, index uint64) (ChunkProof, error) {
	chunks, err := BuildChunks(txs, chunkSize)
	if err != nil {
		return ChunkProof{}, err
	}
	if index >= uint64(len(chunks)) {
		return ChunkProof{}, ErrInvalidChunkProof
	}
	leaves := chunkLeaves(chunks)
	path := merklePath(leaves, index)
	return ChunkProof{
		Commitment: merkleRoot(leaves),
		ChunkSize:  chunkSize,
		ChunkCount: uint64(len(chunks)),
		Index:      index,
		Data:       append([]byte(nil), chunks[index].Data...),
		Path:       path,
	}, nil
}

func VerifyChunkProof(proof ChunkProof) error {
	if proof.ChunkSize == 0 || proof.ChunkCount == 0 || proof.Index >= proof.ChunkCount || uint64(len(proof.Data)) > proof.ChunkSize {
		return ErrInvalidChunkProof
	}
	hash := chunkLeaf(Chunk{Index: proof.Index, Data: proof.Data})
	for _, sibling := range proof.Path {
		if sibling.Left {
			hash = parentHash(sibling.Hash, hash)
		} else {
			hash = parentHash(hash, sibling.Hash)
		}
	}
	if hash != proof.Commitment {
		return ErrInvalidChunkProof
	}
	return nil
}

func RecoverTransactions(proof Proof, chunks []Chunk, parity []Chunk) ([]types.Tx, error) {
	data, err := RecoverData(proof, chunks, parity)
	if err != nil {
		return nil, err
	}
	return decodeTransactions(data)
}

func RecoverData(proof Proof, chunks []Chunk, parity []Chunk) ([]byte, error) {
	if proof.ChunkSize == 0 || proof.ChunkCount == 0 || proof.DataShards == 0 || proof.EncodedSize == 0 {
		return nil, ErrInvalidChunkProof
	}
	if proof.EncodedSize > proof.ChunkSize*proof.ChunkCount {
		return nil, ErrInvalidChunkProof
	}
	dataByIndex := make(map[uint64][]byte, len(chunks))
	for _, chunk := range chunks {
		if chunk.Index >= proof.ChunkCount || uint64(len(chunk.Data)) > proof.ChunkSize {
			return nil, ErrInvalidChunkProof
		}
		dataByIndex[chunk.Index] = append([]byte(nil), chunk.Data...)
	}
	parityByGroup := make(map[uint64][]byte, len(parity))
	for _, chunk := range parity {
		if uint64(len(chunk.Data)) != proof.ChunkSize {
			return nil, ErrInvalidChunkProof
		}
		parityByGroup[chunk.Index] = append([]byte(nil), chunk.Data...)
	}
	groupCount := parityCount(proof.ChunkCount, proof.DataShards)
	for group := uint64(0); group < groupCount; group++ {
		start := group * proof.DataShards
		end := start + proof.DataShards
		if end > proof.ChunkCount {
			end = proof.ChunkCount
		}
		missing := make([]uint64, 0, 1)
		for index := start; index < end; index++ {
			if _, found := dataByIndex[index]; !found {
				missing = append(missing, index)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if len(missing) > 1 {
			return nil, ErrTooManyMissing
		}
		parityData, found := parityByGroup[group]
		if !found {
			return nil, ErrInsufficientChunks
		}
		recovered := append([]byte(nil), parityData...)
		for index := start; index < end; index++ {
			if index == missing[0] {
				continue
			}
			chunkData, found := dataByIndex[index]
			if !found {
				return nil, ErrTooManyMissing
			}
			xorInto(recovered, paddedChunk(chunkData, proof.ChunkSize))
		}
		recovered = trimRecoveredChunk(recovered, missing[0], proof)
		dataByIndex[missing[0]] = recovered
	}
	ordered := make([]Chunk, 0, proof.ChunkCount)
	for index := uint64(0); index < proof.ChunkCount; index++ {
		data, found := dataByIndex[index]
		if !found {
			return nil, ErrInsufficientChunks
		}
		ordered = append(ordered, Chunk{Index: index, Data: append([]byte(nil), data...)})
	}
	if chunkRoot(ordered) != proof.Commitment {
		return nil, ErrCommitmentMismatch
	}
	encoded := make([]byte, 0, proof.EncodedSize)
	for _, chunk := range ordered {
		encoded = append(encoded, chunk.Data...)
	}
	return append([]byte(nil), encoded[:proof.EncodedSize]...), nil
}

func Verify(header types.Header, txs []types.Tx) error {
	if header.ConsensusHash == (types.Hash{}) {
		if len(txs) == 0 {
			return nil
		}
		return ErrMissingData
	}
	if Commitment(txs) != header.ConsensusHash {
		return ErrCommitmentMismatch
	}
	return nil
}

func AttachCommitment(block types.Block) types.Block {
	block.Header.ConsensusHash = Commitment(block.Txs)
	return block
}

func encodeTransactions(txs []types.Tx) []byte {
	size := 8
	for _, tx := range txs {
		size += 8 + len(tx)
	}
	encoded := make([]byte, 0, size)
	encoded = appendUint64(encoded, uint64(len(txs)))
	for _, tx := range txs {
		encoded = appendUint64(encoded, uint64(len(tx)))
		encoded = append(encoded, tx...)
	}
	return encoded
}

func decodeTransactions(data []byte) ([]types.Tx, error) {
	if len(data) < 8 {
		return nil, ErrInvalidEncoding
	}
	count := binary.BigEndian.Uint64(data[:8])
	offset := 8
	txs := make([]types.Tx, 0, count)
	for index := uint64(0); index < count; index++ {
		if len(data)-offset < 8 {
			return nil, ErrInvalidEncoding
		}
		size := binary.BigEndian.Uint64(data[offset : offset+8])
		offset += 8
		if size > uint64(len(data)-offset) {
			return nil, ErrInvalidEncoding
		}
		txs = append(txs, append(types.Tx(nil), data[offset:offset+int(size)]...))
		offset += int(size)
	}
	if offset != len(data) {
		return nil, ErrInvalidEncoding
	}
	return txs, nil
}

func splitChunks(data []byte, chunkSize uint64) ([]Chunk, error) {
	if chunkSize == 0 || chunkSize > uint64(^uint(0)>>1) {
		return nil, ErrInvalidChunkSize
	}
	size := int(chunkSize)
	chunkCount := (len(data) + size - 1) / size
	if chunkCount == 0 {
		chunkCount = 1
	}
	chunks := make([]Chunk, 0, chunkCount)
	for index := 0; index < chunkCount; index++ {
		start := index * size
		end := start + size
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, Chunk{Index: uint64(index), Data: append([]byte(nil), data[start:end]...)})
	}
	return chunks, nil
}

func parityChunks(chunks []Chunk, chunkSize uint64, dataShards uint64) []Chunk {
	if len(chunks) == 0 || dataShards == 0 {
		return nil
	}
	groupCount := parityCount(uint64(len(chunks)), dataShards)
	parity := make([]Chunk, 0, groupCount)
	for group := uint64(0); group < groupCount; group++ {
		start := group * dataShards
		end := start + dataShards
		if end > uint64(len(chunks)) {
			end = uint64(len(chunks))
		}
		buffer := make([]byte, chunkSize)
		for index := start; index < end; index++ {
			xorInto(buffer, paddedChunk(chunks[index].Data, chunkSize))
		}
		parity = append(parity, Chunk{Index: group, Data: buffer})
	}
	return parity
}

func chunkRoot(chunks []Chunk) types.Hash {
	return merkleRoot(chunkLeaves(chunks))
}

func chunkLeaves(chunks []Chunk) []types.Hash {
	ordered := append([]Chunk(nil), chunks...)
	sort.Slice(ordered, func(first int, second int) bool {
		return ordered[first].Index < ordered[second].Index
	})
	leaves := make([]types.Hash, len(ordered))
	for index, chunk := range ordered {
		leaves[index] = chunkLeaf(chunk)
	}
	return leaves
}

func chunkLeaf(chunk Chunk) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte{0})
	writeUint64(hasher, chunk.Index)
	writeUint64(hasher, uint64(len(chunk.Data)))
	hasher.Write(chunk.Data)
	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func merkleRoot(leaves []types.Hash) types.Hash {
	if len(leaves) == 0 {
		return types.Hash{}
	}
	level := append([]types.Hash(nil), leaves...)
	for len(level) > 1 {
		next := make([]types.Hash, 0, (len(level)+1)/2)
		for index := 0; index < len(level); index += 2 {
			left := level[index]
			right := left
			if index+1 < len(level) {
				right = level[index+1]
			}
			next = append(next, parentHash(left, right))
		}
		level = next
	}
	return level[0]
}

func merklePath(leaves []types.Hash, target uint64) []MerkleSibling {
	path := make([]MerkleSibling, 0)
	index := int(target)
	level := append([]types.Hash(nil), leaves...)
	for len(level) > 1 {
		siblingIndex := index ^ 1
		if siblingIndex >= len(level) {
			siblingIndex = index
		}
		path = append(path, MerkleSibling{
			Hash: level[siblingIndex],
			Left: siblingIndex < index,
		})
		next := make([]types.Hash, 0, (len(level)+1)/2)
		for position := 0; position < len(level); position += 2 {
			left := level[position]
			right := left
			if position+1 < len(level) {
				right = level[position+1]
			}
			next = append(next, parentHash(left, right))
		}
		index /= 2
		level = next
	}
	return path
}

func parentHash(left types.Hash, right types.Hash) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte{1})
	hasher.Write(left[:])
	hasher.Write(right[:])
	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func paddedChunk(data []byte, chunkSize uint64) []byte {
	padded := make([]byte, chunkSize)
	copy(padded, data)
	return padded
}

func xorInto(target []byte, source []byte) {
	for index := range target {
		target[index] ^= source[index]
	}
}

func trimRecoveredChunk(data []byte, index uint64, proof Proof) []byte {
	expected := proof.ChunkSize
	if index == proof.ChunkCount-1 {
		used := proof.EncodedSize - proof.ChunkSize*(proof.ChunkCount-1)
		if used < expected {
			expected = used
		}
	}
	return append([]byte(nil), data[:expected]...)
}

func parityCount(chunkCount uint64, dataShards uint64) uint64 {
	if chunkCount == 0 || dataShards == 0 {
		return 0
	}
	return (chunkCount + dataShards - 1) / dataShards
}

func appendUint64(buffer []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(buffer, encoded[:]...)
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}
