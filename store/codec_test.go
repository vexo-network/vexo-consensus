package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

// fullBlockRecord sets every field of a block record, so a field the codec drops shows as a
// difference after a round trip.
func fullBlockRecord() BlockRecord {
	quorumCert := QuorumCertRecord{Height: 7, Round: 9, BlockHash: types.Hash{1}, Signers: types.Bitmap{0b101}, Signature: types.AggregateSignature("sig"), VotingPower: 3}
	return BlockRecord{
		Block: types.Block{
			Header: types.Header{
				ChainID: "vexo", Height: 7, TimeUnixNano: -5, PreviousBlockHash: types.Hash{2}, AppHash: types.Hash{3},
				ValidatorSetHash: types.Hash{4}, NextValidatorSetHash: types.Hash{5}, ConsensusHash: types.Hash{6}, TxRoot: types.Hash{7},
				Round: 9, JustifyRound: 8, ValidatorSetHeight: 6, StateHeight: 5,
				Benched: []types.Bench{{Validator: "carol", Until: 1009}},
			},
			Txs: []types.Tx{types.Tx("a"), types.Tx("bb")},
		},
		Hash:       types.Hash{1},
		AppHash:    types.Hash{8},
		QuorumCert: &quorumCert,
		StateRoots: []StateRootRecord{{Height: 7, Namespace: "bank", Root: types.Hash{9}}},
		TxResults:  []types.Result{{Code: 4, Log: "out of gas", Data: []byte{1, 2}, GasUsed: 21_000, FeePaid: 42}},
	}
}

func fullFinalityProofRecord() FinalityProofRecord {
	block := fullBlockRecord()
	return FinalityProofRecord{
		Header:             block.Block.Header,
		BlockHash:          block.Hash,
		QuorumCert:         *block.QuorumCert,
		CommitChain:        []CommitLinkRecord{{Header: block.Block.Header, BlockHash: types.Hash{10}, QuorumCert: *block.QuorumCert}},
		ValidatorSetHeight: 6,
		ValidatorSetHash:   types.Hash{4},
	}
}

// storeRecordCodecs are the encode and decode functions of every record the store keeps, with a
// value that sets each field.
func storeRecordCodecs() map[string]struct {
	value  any
	encode func() []byte
	decode func([]byte) (any, error)
} {
	type codec = struct {
		value  any
		encode func() []byte
		decode func([]byte) (any, error)
	}
	wrap := func(value any, encode func() []byte, decode func([]byte) (any, error)) codec {
		return codec{value: value, encode: encode, decode: decode}
	}
	block := fullBlockRecord()
	index := BlockIndex{EarliestHeight: 2, LatestHeight: 9, TotalBlocks: 8}
	state := StateRecord{Height: 9, AppHash: types.Hash{1}, LastBlockHash: types.Hash{2}, ValidatorSetHash: types.Hash{3}, BaseFee: 4, NextBaseFee: 5, BlobBaseFee: 6, NextBlobBaseFee: 7, BlobGasUsed: 8, ExcessBlobGas: 9}
	root := StateRootRecord{Height: 9, Namespace: "staking", Root: types.Hash{4}}
	proof := fullFinalityProofRecord()
	evidence := EvidenceRecord{Evidence: slashing.Evidence{Type: slashing.EvidenceConflictingVote, Validator: "bob", Height: 3, Round: 2, Proof: []byte("proof")}}
	schema := upgrade.State{Height: 1, BinaryVersion: "v1", ConfigSchemaVersion: 2, StoreSchemaVersion: 3, AppStateVersion: 4}
	plan := upgrade.Plan{Name: "v2", Height: 100, BinaryVersion: "v2", ConfigSchemaFrom: 1, ConfigSchemaTo: 2, StoreSchemaFrom: 3, StoreSchemaTo: 4, AppStateSchemaFrom: 5, AppStateSchemaTo: 6, GovernanceProposal: "p1", RollbackBinary: "v1", AllowNoopMigrations: true}
	history := kvHistoryRecord{Height: 5, Namespace: "bank", Key: []byte("k"), Value: []byte("v"), Deleted: true}
	return map[string]codec{
		"block":    wrap(block, func() []byte { return encodeBlockRecord(block) }, func(data []byte) (any, error) { return decodeBlockRecord(data) }),
		"index":    wrap(index, func() []byte { return encodeBlockIndex(index) }, func(data []byte) (any, error) { return decodeBlockIndex(data) }),
		"state":    wrap(state, func() []byte { return encodeStateRecord(state) }, func(data []byte) (any, error) { return decodeStateRecord(data) }),
		"root":     wrap(root, func() []byte { return encodeStateRootRecord(root) }, func(data []byte) (any, error) { return decodeStateRootRecord(data) }),
		"proof":    wrap(proof, func() []byte { return encodeFinalityProofRecord(proof) }, func(data []byte) (any, error) { return decodeFinalityProofRecord(data) }),
		"evidence": wrap(evidence, func() []byte { return encodeEvidenceRecord(evidence) }, func(data []byte) (any, error) { return decodeEvidenceRecord(data) }),
		"schema":   wrap(schema, func() []byte { return encodeSchemaState(schema) }, func(data []byte) (any, error) { return decodeSchemaState(data) }),
		"plan":     wrap(plan, func() []byte { return encodeUpgradePlan(plan) }, func(data []byte) (any, error) { return decodeUpgradePlanRecord(data) }),
		"history":  wrap(history, func() []byte { return encodeKVHistoryRecord(history) }, func(data []byte) (any, error) { return decodeKVHistoryRecord(data) }),
	}
}

// Every record round-trips with every field, and its decoder refuses a truncated record and one
// with bytes over.
func TestStoreRecordsRoundTripAndRefuseDamagedBytes(t *testing.T) {
	for name, codec := range storeRecordCodecs() {
		t.Run(name, func(t *testing.T) {
			encoded := codec.encode()
			decoded, err := codec.decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, codec.value) {
				t.Fatalf("round trip changed the record:\n got %+v\nwant %+v", decoded, codec.value)
			}
			if _, err := codec.decode(append(append([]byte(nil), encoded...), 0)); !errors.Is(err, types.ErrInvalidEncoding) {
				t.Fatalf("expected trailing bytes refused, got %v", err)
			}
			if _, err := codec.decode(encoded[:len(encoded)-1]); !errors.Is(err, types.ErrInvalidEncoding) {
				t.Fatalf("expected a truncated record refused, got %v", err)
			}
		})
	}
}

// The codecs cover every field of the records they encode: a field added to a record without its
// codec is caught here, since fullBlockRecord and the other values set every field they know of.
func TestStoreRecordValuesSetEveryField(t *testing.T) {
	for name, codec := range storeRecordCodecs() {
		value := reflect.ValueOf(codec.value)
		for index := range value.NumField() {
			if value.Field(index).IsZero() {
				t.Errorf("%s: the test value leaves %s zero, so its round trip proves nothing", name, value.Type().Field(index).Name)
			}
		}
	}
}

// A record that does not use a presence byte of 0 or 1 is refused rather than read as another.
func TestBlockRecordRefusesAnInvalidPresenceByte(t *testing.T) {
	record := fullBlockRecord()
	record.QuorumCert = nil
	encoded := encodeBlockRecord(record)
	presence := types.BlockSize(record.Block) + 2*len(types.Hash{})
	encoded[presence] = 2
	if _, err := decodeBlockRecord(encoded); !errors.Is(err, types.ErrInvalidEncoding) {
		t.Fatalf("expected an invalid presence byte refused, got %v", err)
	}
}

// Decoding arbitrary bytes never panics, and whatever decodes encodes back to the same bytes: a
// record has one encoding.
func FuzzDecodeStoreRecords(f *testing.F) {
	for _, codec := range storeRecordCodecs() {
		f.Add(codec.encode())
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if record, err := decodeBlockRecord(data); err == nil && string(encodeBlockRecord(record)) != string(data) {
			t.Fatal("block record re-encodes differently")
		}
		if record, err := decodeFinalityProofRecord(data); err == nil && string(encodeFinalityProofRecord(record)) != string(data) {
			t.Fatal("finality proof re-encodes differently")
		}
		if record, err := decodeKVHistoryRecord(data); err == nil && string(encodeKVHistoryRecord(record)) != string(data) {
			t.Fatal("history record re-encodes differently")
		}
		if record, err := decodeUpgradePlanRecord(data); err == nil && string(encodeUpgradePlan(record)) != string(data) {
			t.Fatal("upgrade plan re-encodes differently")
		}
		if record, err := decodeEvidenceRecord(data); err == nil && string(encodeEvidenceRecord(record)) != string(data) {
			t.Fatal("evidence record re-encodes differently")
		}
	})
}
