package store

import (
	"encoding/binary"
	"math"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

// The records of the store are kept in the canonical binary encoding of package types: minimal
// uvarints, length-prefixed byte strings and lists, hashes as their 32 bytes, fields in the order
// of the struct that declares them. A decoder refuses anything else, trailing bytes included, so a
// damaged record is reported rather than read as another value.
//
// The store reads only what it wrote itself, so the limits a decoder checks lengths against bound
// memory, not policy: a block, a log or a value of any size the node accepted reads back.
const (
	maxStoredBytes     = math.MaxInt32
	maxStoredBlockCost = math.MaxInt32
	maxStoredList      = math.MaxInt32
)

// decodeRecord reads data with read and refuses trailing bytes.
func decodeRecord[Record any](data []byte, read func(*types.Reader) Record) (Record, error) {
	reader := types.NewReader(data)
	record := read(reader)
	if err := reader.Done(); err != nil {
		var zero Record
		return zero, err
	}
	return record, nil
}

func appendUvarint[Value ~uint64 | ~uint32](dst []byte, value Value) []byte {
	return binary.AppendUvarint(dst, uint64(value))
}

func appendBool(dst []byte, value bool) []byte {
	if value {
		return append(dst, 1)
	}
	return append(dst, 0)
}

func readUint32(reader *types.Reader) uint32 {
	value := reader.Uvarint()
	if value > math.MaxUint32 {
		reader.Fail("value overflows 32 bits")
		return 0
	}
	return uint32(value)
}

func appendQuorumCertRecord(dst []byte, record QuorumCertRecord) []byte {
	dst = appendUvarint(dst, record.Height)
	dst = appendUvarint(dst, record.Round)
	dst = append(dst, record.BlockHash[:]...)
	dst = types.AppendBytes(dst, record.Signers)
	dst = types.AppendBytes(dst, record.Signature)
	return appendUvarint(dst, record.VotingPower)
}

func readQuorumCertRecord(reader *types.Reader) QuorumCertRecord {
	var record QuorumCertRecord
	record.Height = types.Height(reader.Uvarint())
	record.Round = types.Round(reader.Uvarint())
	record.BlockHash = reader.Hash()
	record.Signers = types.Bitmap(reader.Bytes(maxStoredBytes))
	record.Signature = types.AggregateSignature(reader.Bytes(maxStoredBytes))
	record.VotingPower = types.VotingPower(reader.Uvarint())
	return record
}

func appendStateRootRecord(dst []byte, record StateRootRecord) []byte {
	dst = appendUvarint(dst, record.Height)
	dst = types.AppendBytes(dst, record.Namespace)
	return append(dst, record.Root[:]...)
}

func readStateRootRecord(reader *types.Reader) StateRootRecord {
	var record StateRootRecord
	record.Height = types.Height(reader.Uvarint())
	record.Namespace = reader.String(maxStoredBytes)
	record.Root = reader.Hash()
	return record
}

func appendResult(dst []byte, result types.Result) []byte {
	dst = appendUvarint(dst, result.Code)
	dst = types.AppendBytes(dst, result.Log)
	dst = types.AppendBytes(dst, result.Data)
	dst = appendUvarint(dst, result.GasUsed)
	return appendUvarint(dst, result.FeePaid)
}

func readResult(reader *types.Reader) types.Result {
	var result types.Result
	result.Code = readUint32(reader)
	result.Log = reader.String(maxStoredBytes)
	result.Data = reader.Bytes(maxStoredBytes)
	result.GasUsed = reader.Uvarint()
	result.FeePaid = reader.Uvarint()
	return result
}

// encodeBlockRecord encodes the block, its hash and app hash, its certificate behind a presence
// byte, its state roots and its transaction results.
func encodeBlockRecord(record BlockRecord) []byte {
	dst := types.AppendBlock(make([]byte, 0, types.BlockSize(record.Block)+128), record.Block)
	dst = append(dst, record.Hash[:]...)
	dst = append(dst, record.AppHash[:]...)
	dst = appendBool(dst, record.QuorumCert != nil)
	if record.QuorumCert != nil {
		dst = appendQuorumCertRecord(dst, *record.QuorumCert)
	}
	dst = appendUvarint(dst, uint64(len(record.StateRoots)))
	for _, root := range record.StateRoots {
		dst = appendStateRootRecord(dst, root)
	}
	dst = appendUvarint(dst, uint64(len(record.TxResults)))
	for _, result := range record.TxResults {
		dst = appendResult(dst, result)
	}
	return dst
}

func decodeBlockRecord(data []byte) (BlockRecord, error) {
	return decodeRecord(data, func(reader *types.Reader) BlockRecord {
		var record BlockRecord
		record.Block = reader.Block(maxStoredBlockCost)
		record.Hash = reader.Hash()
		record.AppHash = reader.Hash()
		if reader.Bool() {
			quorumCert := readQuorumCertRecord(reader)
			record.QuorumCert = &quorumCert
		}
		if count := reader.Count(maxStoredList, 1); count > 0 {
			record.StateRoots = make([]StateRootRecord, count)
			for index := range record.StateRoots {
				record.StateRoots[index] = readStateRootRecord(reader)
			}
		}
		if count := reader.Count(maxStoredList, 1); count > 0 {
			record.TxResults = make([]types.Result, count)
			for index := range record.TxResults {
				record.TxResults[index] = readResult(reader)
			}
		}
		return record
	})
}

func encodeBlockIndex(index BlockIndex) []byte {
	dst := appendUvarint(nil, index.EarliestHeight)
	dst = appendUvarint(dst, index.LatestHeight)
	return appendUvarint(dst, index.TotalBlocks)
}

func decodeBlockIndex(data []byte) (BlockIndex, error) {
	return decodeRecord(data, func(reader *types.Reader) BlockIndex {
		return BlockIndex{
			EarliestHeight: types.Height(reader.Uvarint()),
			LatestHeight:   types.Height(reader.Uvarint()),
			TotalBlocks:    reader.Uvarint(),
		}
	})
}

func encodeStateRecord(state StateRecord) []byte {
	dst := appendUvarint(nil, state.Height)
	dst = append(dst, state.AppHash[:]...)
	dst = append(dst, state.LastBlockHash[:]...)
	dst = append(dst, state.ValidatorSetHash[:]...)
	dst = appendUvarint(dst, state.BaseFee)
	dst = appendUvarint(dst, state.NextBaseFee)
	dst = appendUvarint(dst, state.BlobBaseFee)
	dst = appendUvarint(dst, state.NextBlobBaseFee)
	dst = appendUvarint(dst, state.BlobGasUsed)
	return appendUvarint(dst, state.ExcessBlobGas)
}

func decodeStateRecord(data []byte) (StateRecord, error) {
	return decodeRecord(data, func(reader *types.Reader) StateRecord {
		return StateRecord{
			Height:           types.Height(reader.Uvarint()),
			AppHash:          reader.Hash(),
			LastBlockHash:    reader.Hash(),
			ValidatorSetHash: reader.Hash(),
			BaseFee:          reader.Uvarint(),
			NextBaseFee:      reader.Uvarint(),
			BlobBaseFee:      reader.Uvarint(),
			NextBlobBaseFee:  reader.Uvarint(),
			BlobGasUsed:      reader.Uvarint(),
			ExcessBlobGas:    reader.Uvarint(),
		}
	})
}

func encodeStateRootRecord(record StateRootRecord) []byte {
	return appendStateRootRecord(nil, record)
}

func decodeStateRootRecord(data []byte) (StateRootRecord, error) {
	return decodeRecord(data, readStateRootRecord)
}

func encodeFinalityProofRecord(proof FinalityProofRecord) []byte {
	dst := types.AppendHeader(nil, proof.Header)
	dst = append(dst, proof.BlockHash[:]...)
	dst = appendQuorumCertRecord(dst, proof.QuorumCert)
	dst = appendUvarint(dst, uint64(len(proof.CommitChain)))
	for _, link := range proof.CommitChain {
		dst = types.AppendHeader(dst, link.Header)
		dst = append(dst, link.BlockHash[:]...)
		dst = appendQuorumCertRecord(dst, link.QuorumCert)
	}
	dst = appendUvarint(dst, proof.ValidatorSetHeight)
	return append(dst, proof.ValidatorSetHash[:]...)
}

func decodeFinalityProofRecord(data []byte) (FinalityProofRecord, error) {
	return decodeRecord(data, func(reader *types.Reader) FinalityProofRecord {
		var proof FinalityProofRecord
		proof.Header = reader.Header()
		proof.BlockHash = reader.Hash()
		proof.QuorumCert = readQuorumCertRecord(reader)
		if count := reader.Count(maxStoredList, 1); count > 0 {
			proof.CommitChain = make([]CommitLinkRecord, count)
			for index := range proof.CommitChain {
				proof.CommitChain[index] = CommitLinkRecord{
					Header:     reader.Header(),
					BlockHash:  reader.Hash(),
					QuorumCert: readQuorumCertRecord(reader),
				}
			}
		}
		proof.ValidatorSetHeight = types.Height(reader.Uvarint())
		proof.ValidatorSetHash = reader.Hash()
		return proof
	})
}

func encodeEvidenceRecord(record EvidenceRecord) []byte {
	evidence := record.Evidence
	dst := types.AppendBytes(nil, evidence.Type)
	dst = types.AppendBytes(dst, evidence.Validator)
	dst = appendUvarint(dst, evidence.Height)
	dst = appendUvarint(dst, evidence.Round)
	return types.AppendBytes(dst, evidence.Proof)
}

func decodeEvidenceRecord(data []byte) (EvidenceRecord, error) {
	return decodeRecord(data, func(reader *types.Reader) EvidenceRecord {
		return EvidenceRecord{Evidence: slashing.Evidence{
			Type:      slashing.EvidenceType(reader.String(maxStoredBytes)),
			Validator: types.ValidatorID(reader.String(maxStoredBytes)),
			Height:    types.Height(reader.Uvarint()),
			Round:     types.Round(reader.Uvarint()),
			Proof:     reader.Bytes(maxStoredBytes),
		}}
	})
}

func encodeSchemaState(state upgrade.State) []byte {
	dst := appendUvarint(nil, state.Height)
	dst = types.AppendBytes(dst, state.BinaryVersion)
	dst = appendUvarint(dst, state.ConfigSchemaVersion)
	dst = appendUvarint(dst, state.StoreSchemaVersion)
	return appendUvarint(dst, state.AppStateVersion)
}

func decodeSchemaState(data []byte) (upgrade.State, error) {
	return decodeRecord(data, func(reader *types.Reader) upgrade.State {
		return upgrade.State{
			Height:              types.Height(reader.Uvarint()),
			BinaryVersion:       reader.String(maxStoredBytes),
			ConfigSchemaVersion: reader.Uvarint(),
			StoreSchemaVersion:  reader.Uvarint(),
			AppStateVersion:     reader.Uvarint(),
		}
	})
}

func encodeUpgradePlan(plan upgrade.Plan) []byte {
	dst := types.AppendBytes(nil, plan.Name)
	dst = appendUvarint(dst, plan.Height)
	dst = types.AppendBytes(dst, plan.BinaryVersion)
	dst = appendUvarint(dst, plan.ConfigSchemaFrom)
	dst = appendUvarint(dst, plan.ConfigSchemaTo)
	dst = appendUvarint(dst, plan.StoreSchemaFrom)
	dst = appendUvarint(dst, plan.StoreSchemaTo)
	dst = appendUvarint(dst, plan.AppStateSchemaFrom)
	dst = appendUvarint(dst, plan.AppStateSchemaTo)
	dst = types.AppendBytes(dst, plan.GovernanceProposal)
	dst = types.AppendBytes(dst, plan.RollbackBinary)
	return appendBool(dst, plan.AllowNoopMigrations)
}

func decodeUpgradePlanRecord(data []byte) (upgrade.Plan, error) {
	return decodeRecord(data, func(reader *types.Reader) upgrade.Plan {
		return upgrade.Plan{
			Name:                reader.String(maxStoredBytes),
			Height:              types.Height(reader.Uvarint()),
			BinaryVersion:       reader.String(maxStoredBytes),
			ConfigSchemaFrom:    reader.Uvarint(),
			ConfigSchemaTo:      reader.Uvarint(),
			StoreSchemaFrom:     reader.Uvarint(),
			StoreSchemaTo:       reader.Uvarint(),
			AppStateSchemaFrom:  reader.Uvarint(),
			AppStateSchemaTo:    reader.Uvarint(),
			GovernanceProposal:  reader.String(maxStoredBytes),
			RollbackBinary:      reader.String(maxStoredBytes),
			AllowNoopMigrations: reader.Bool(),
		}
	})
}

func encodeKVHistoryRecord(record kvHistoryRecord) []byte {
	dst := appendUvarint(nil, record.Height)
	dst = types.AppendBytes(dst, record.Namespace)
	dst = types.AppendBytes(dst, record.Key)
	dst = types.AppendBytes(dst, record.Value)
	return appendBool(dst, record.Deleted)
}

func decodeKVHistoryRecord(data []byte) (kvHistoryRecord, error) {
	return decodeRecord(data, func(reader *types.Reader) kvHistoryRecord {
		return kvHistoryRecord{
			Height:    types.Height(reader.Uvarint()),
			Namespace: reader.String(maxStoredBytes),
			Key:       reader.Bytes(maxStoredBytes),
			Value:     reader.Bytes(maxStoredBytes),
			Deleted:   reader.Bool(),
		}
	})
}
