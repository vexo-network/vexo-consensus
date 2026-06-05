package dataavailability

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrCommitmentMismatch  = errors.New("data availability commitment mismatch")
	ErrMissingData         = errors.New("data availability data is missing")
	ErrInvalidChunkSize    = errors.New("invalid data availability chunk size")
	ErrInvalidDataShards   = errors.New("invalid data availability data shards")
	ErrInvalidParityShards = errors.New("invalid data availability parity shards")
	ErrInvalidChunkProof   = errors.New("invalid data availability chunk proof")
	ErrInsufficientChunks  = errors.New("insufficient data availability chunks")
	ErrTooManyMissing      = errors.New("too many missing data availability chunks")
	ErrInvalidEncoding     = errors.New("invalid data availability transaction encoding")
)

const (
	DefaultChunkSize    uint64 = 1024
	DefaultDataShards   uint64 = 8
	DefaultParityShards uint64 = 2
	maxErasureShards    uint64 = 255
)

type Proof struct {
	Commitment   types.Hash
	TxCount      uint64
	TotalBytes   uint64
	EncodedSize  uint64
	ChunkSize    uint64
	ChunkCount   uint64
	DataShards   uint64
	ParityShards uint64
	ParityCount  uint64
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
	return BuildProofWithErasureOptions(txs, chunkSize, dataShards, DefaultParityShards)
}

func BuildProofWithErasureOptions(txs []types.Tx, chunkSize uint64, dataShards uint64, parityShards uint64) (Proof, error) {
	if dataShards == 0 || dataShards > maxErasureShards {
		return Proof{}, ErrInvalidDataShards
	}
	if parityShards == 0 || parityShards > maxErasureShards {
		return Proof{}, ErrInvalidParityShards
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
		Commitment:   chunkRoot(chunks),
		TxCount:      uint64(len(txs)),
		TotalBytes:   totalBytes,
		EncodedSize:  uint64(len(encoded)),
		ChunkSize:    chunkSize,
		ChunkCount:   uint64(len(chunks)),
		DataShards:   dataShards,
		ParityShards: parityShards,
		ParityCount:  parityGroupCount(uint64(len(chunks)), dataShards) * parityShards,
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
	return BuildParityChunksWithOptions(txs, chunkSize, dataShards, DefaultParityShards)
}

func BuildParityChunksWithOptions(txs []types.Tx, chunkSize uint64, dataShards uint64, parityShards uint64) ([]Chunk, error) {
	if dataShards == 0 || dataShards > maxErasureShards {
		return nil, ErrInvalidDataShards
	}
	if parityShards == 0 || parityShards > maxErasureShards {
		return nil, ErrInvalidParityShards
	}
	chunks, err := BuildChunks(txs, chunkSize)
	if err != nil {
		return nil, err
	}
	return reedSolomonParityChunks(chunks, chunkSize, dataShards, parityShards), nil
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
	parityShards := proof.ParityShards
	if parityShards == 0 {
		parityShards = 1
	}
	if proof.ChunkSize == 0 || proof.ChunkCount == 0 || proof.DataShards == 0 || proof.EncodedSize == 0 || parityShards == 0 {
		return nil, ErrInvalidChunkProof
	}
	if proof.DataShards > maxErasureShards || parityShards > maxErasureShards {
		return nil, ErrInvalidChunkProof
	}
	if proof.EncodedSize > proof.ChunkSize*proof.ChunkCount {
		return nil, ErrInvalidChunkProof
	}
	groupCount := parityGroupCount(proof.ChunkCount, proof.DataShards)
	if proof.ParityCount != 0 && proof.ParityCount != groupCount*parityShards {
		return nil, ErrInvalidChunkProof
	}
	dataByIndex := make(map[uint64][]byte, len(chunks))
	for _, chunk := range chunks {
		if chunk.Index >= proof.ChunkCount || uint64(len(chunk.Data)) > proof.ChunkSize {
			return nil, ErrInvalidChunkProof
		}
		if _, found := dataByIndex[chunk.Index]; found {
			return nil, ErrInvalidChunkProof
		}
		dataByIndex[chunk.Index] = append([]byte(nil), chunk.Data...)
	}
	parityByGroup := make(map[uint64]map[uint64][]byte, len(parity))
	for _, chunk := range parity {
		if uint64(len(chunk.Data)) != proof.ChunkSize {
			return nil, ErrInvalidChunkProof
		}
		group := chunk.Index / parityShards
		shard := chunk.Index % parityShards
		if group >= parityGroupCount(proof.ChunkCount, proof.DataShards) {
			return nil, ErrInvalidChunkProof
		}
		if parityByGroup[group] == nil {
			parityByGroup[group] = make(map[uint64][]byte)
		}
		if _, found := parityByGroup[group][shard]; found {
			return nil, ErrInvalidChunkProof
		}
		parityByGroup[group][shard] = append([]byte(nil), chunk.Data...)
	}
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
		if uint64(len(missing)) > parityShards {
			return nil, ErrTooManyMissing
		}
		paritySet := parityByGroup[group]
		if len(paritySet) < len(missing) {
			return nil, ErrInsufficientChunks
		}
		recovered, err := recoverMissingGroupChunks(start, end, proof.ChunkSize, missing, dataByIndex, paritySet)
		if err != nil {
			return nil, err
		}
		for index, data := range recovered {
			dataByIndex[index] = trimRecoveredChunk(data, index, proof)
		}
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

func reedSolomonParityChunks(chunks []Chunk, chunkSize uint64, dataShards uint64, parityShards uint64) []Chunk {
	if len(chunks) == 0 || dataShards == 0 || parityShards == 0 {
		return nil
	}
	groupCount := parityGroupCount(uint64(len(chunks)), dataShards)
	parity := make([]Chunk, 0, groupCount*parityShards)
	for group := uint64(0); group < groupCount; group++ {
		start := group * dataShards
		end := start + dataShards
		if end > uint64(len(chunks)) {
			end = uint64(len(chunks))
		}
		for parityShard := uint64(0); parityShard < parityShards; parityShard++ {
			buffer := make([]byte, chunkSize)
			x := byte(parityShard + 1)
			for index := start; index < end; index++ {
				coefficient := gfPow(x, index-start)
				xorMulInto(buffer, paddedChunk(chunks[index].Data, chunkSize), coefficient)
			}
			parity = append(parity, Chunk{Index: group*parityShards + parityShard, Data: buffer})
		}
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

func xorMulInto(target []byte, source []byte, coefficient byte) {
	if coefficient == 0 {
		return
	}
	if coefficient == 1 {
		xorInto(target, source)
		return
	}
	for index := range target {
		target[index] ^= gfMul(source[index], coefficient)
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

func parityGroupCount(chunkCount uint64, dataShards uint64) uint64 {
	if chunkCount == 0 || dataShards == 0 {
		return 0
	}
	return (chunkCount + dataShards - 1) / dataShards
}

func recoverMissingGroupChunks(start uint64, end uint64, chunkSize uint64, missing []uint64, dataByIndex map[uint64][]byte, paritySet map[uint64][]byte) (map[uint64][]byte, error) {
	equationShards := sortedParityShards(paritySet)
	if len(equationShards) < len(missing) {
		return nil, ErrInsufficientChunks
	}
	equationShards = equationShards[:len(missing)]
	matrix := make([][]byte, len(missing))
	right := make([][]byte, len(missing))
	for row, parityShard := range equationShards {
		x := byte(parityShard + 1)
		adjusted := append([]byte(nil), paritySet[parityShard]...)
		for index := start; index < end; index++ {
			if containsMissing(missing, index) {
				continue
			}
			data, found := dataByIndex[index]
			if !found {
				return nil, ErrTooManyMissing
			}
			coefficient := gfPow(x, index-start)
			xorMulInto(adjusted, paddedChunk(data, chunkSize), coefficient)
		}
		matrix[row] = make([]byte, len(missing))
		for column, missingIndex := range missing {
			matrix[row][column] = gfPow(x, missingIndex-start)
		}
		right[row] = adjusted
	}
	inverse, err := invertMatrix(matrix)
	if err != nil {
		return nil, ErrInsufficientChunks
	}
	recovered := make(map[uint64][]byte, len(missing))
	for column, missingIndex := range missing {
		data := make([]byte, chunkSize)
		for row := range right {
			xorMulInto(data, right[row], inverse[column][row])
		}
		recovered[missingIndex] = data
	}
	return recovered, nil
}

func sortedParityShards(paritySet map[uint64][]byte) []uint64 {
	shards := make([]uint64, 0, len(paritySet))
	for shard := range paritySet {
		shards = append(shards, shard)
	}
	sort.Slice(shards, func(first int, second int) bool {
		return shards[first] < shards[second]
	})
	return shards
}

func containsMissing(missing []uint64, value uint64) bool {
	for _, candidate := range missing {
		if candidate == value {
			return true
		}
	}
	return false
}

func invertMatrix(matrix [][]byte) ([][]byte, error) {
	size := len(matrix)
	if size == 0 {
		return nil, ErrInsufficientChunks
	}
	augmented := make([][]byte, size)
	for row := 0; row < size; row++ {
		if len(matrix[row]) != size {
			return nil, ErrInsufficientChunks
		}
		augmented[row] = make([]byte, size*2)
		copy(augmented[row], matrix[row])
		augmented[row][size+row] = 1
	}
	for column := 0; column < size; column++ {
		pivot := column
		for pivot < size && augmented[pivot][column] == 0 {
			pivot++
		}
		if pivot == size {
			return nil, ErrInsufficientChunks
		}
		if pivot != column {
			augmented[pivot], augmented[column] = augmented[column], augmented[pivot]
		}
		inversePivot := gfInv(augmented[column][column])
		for index := range augmented[column] {
			augmented[column][index] = gfMul(augmented[column][index], inversePivot)
		}
		for row := 0; row < size; row++ {
			if row == column || augmented[row][column] == 0 {
				continue
			}
			factor := augmented[row][column]
			for index := range augmented[row] {
				augmented[row][index] ^= gfMul(factor, augmented[column][index])
			}
		}
	}
	inverse := make([][]byte, size)
	for row := 0; row < size; row++ {
		inverse[row] = append([]byte(nil), augmented[row][size:]...)
	}
	return inverse, nil
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

var gfExpTable, gfLogTable = buildGFTables()

func buildGFTables() ([512]byte, [256]byte) {
	var exp [512]byte
	var log [256]byte
	value := 1
	for index := 0; index < 255; index++ {
		exp[index] = byte(value)
		log[byte(value)] = byte(index)
		value <<= 1
		if value&0x100 != 0 {
			value ^= 0x11d
		}
	}
	for index := 255; index < len(exp); index++ {
		exp[index] = exp[index-255]
	}
	return exp, log
}

func gfMul(left byte, right byte) byte {
	if left == 0 || right == 0 {
		return 0
	}
	return gfExpTable[int(gfLogTable[left])+int(gfLogTable[right])]
}

func gfPow(value byte, power uint64) byte {
	if power == 0 {
		return 1
	}
	if value == 0 {
		return 0
	}
	logValue := uint64(gfLogTable[value])
	return gfExpTable[(logValue*(power%255))%255]
}

func gfInv(value byte) byte {
	if value == 0 {
		return 0
	}
	return gfExpTable[255-int(gfLogTable[value])]
}
