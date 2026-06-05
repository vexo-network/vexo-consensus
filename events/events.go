package events

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const Namespace = "events"

var (
	ErrInvalidEvent = errors.New("invalid event")
	ErrStoreMissing = errors.New("event store is required")
)

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Index bool   `json:"index,omitempty"`
}

type Event struct {
	Type       string      `json:"type"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

type Record struct {
	Height  uint64 `json:"height"`
	TxIndex int    `json:"tx_index"`
	Event   Event  `json:"event"`
}

type Sink interface {
	Emit(event Event) error
}

type Indexer struct {
	store store.KVStore
}

func NewIndexer(store store.KVStore) *Indexer {
	return &Indexer{store: store}
}

func (indexer *Indexer) IndexBlock(ctx context.Context, height types.Height, txEvents [][]Event) error {
	if indexer == nil || indexer.store == nil {
		return ErrStoreMissing
	}
	writes := make([]kvbatch.KVWrite, 0)
	for txIndex, events := range txEvents {
		for _, event := range events {
			if event.Type == "" {
				return ErrInvalidEvent
			}
			record := Record{Height: uint64(height), TxIndex: txIndex, Event: cloneEvent(event)}
			encoded, err := json.Marshal(record)
			if err != nil {
				return err
			}
			writes = append(writes, kvbatch.KVWrite{Namespace: Namespace, Key: eventKey(height, txIndex, event.Type, len(writes)), Value: encoded})
			for _, attribute := range event.Attributes {
				if !attribute.Index || attribute.Key == "" {
					continue
				}
				writes = append(writes, kvbatch.KVWrite{Namespace: Namespace, Key: attributeKey(attribute.Key, attribute.Value, height, txIndex, event.Type), Value: encoded})
				writes = append(writes, kvbatch.KVWrite{Namespace: Namespace, Key: legacyAttributeKey(attribute.Key, attribute.Value, height, txIndex, event.Type), Value: encoded})
			}
		}
	}
	if len(writes) == 0 {
		return nil
	}
	if batch, ok := indexer.store.(kvbatch.BatchKVStore); ok {
		return batch.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := indexer.store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) Query(ctx context.Context, key string, value string) ([]Record, error) {
	if indexer == nil || indexer.store == nil {
		return nil, ErrStoreMissing
	}
	prefixStore, ok := indexer.store.(store.PrefixKVStore)
	if !ok {
		return indexer.queryLegacy(ctx, key, value)
	}
	pairs, err := prefixStore.ExportPrefix(ctx, Namespace, attributePrefix(key, value))
	if err != nil {
		return nil, err
	}
	records, err := decodeRecords(pairs)
	if err != nil {
		return nil, err
	}
	if len(records) > 0 {
		return records, nil
	}
	return indexer.queryLegacy(ctx, key, value)
}

func (indexer *Indexer) queryLegacy(ctx context.Context, key string, value string) ([]Record, error) {
	snapshot, ok := indexer.store.(store.SnapshotKVStore)
	if !ok {
		return nil, ErrStoreMissing
	}
	prefix := []byte("attr/" + key + "/" + value + "/")
	pairs, err := snapshot.ExportNamespace(ctx, Namespace)
	if err != nil {
		return nil, err
	}
	filtered := make([]store.KVPair, 0)
	for _, pair := range pairs {
		if bytes.HasPrefix(pair.Key, prefix) {
			filtered = append(filtered, pair)
		}
	}
	return decodeRecords(filtered)
}

func decodeRecords(pairs []store.KVPair) ([]Record, error) {
	records := make([]Record, 0, len(pairs))
	seen := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		if _, found := seen[string(pair.Value)]; found {
			continue
		}
		seen[string(pair.Value)] = struct{}{}
		var record Record
		if err := json.Unmarshal(pair.Value, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(left, right int) bool {
		if records[left].Height != records[right].Height {
			return records[left].Height < records[right].Height
		}
		return records[left].TxIndex < records[right].TxIndex
	})
	return records, nil
}

func eventKey(height types.Height, txIndex int, eventType string, sequence int) []byte {
	return []byte("height/" + strconv.FormatUint(uint64(height), 10) + "/" + strconv.Itoa(txIndex) + "/" + eventType + "/" + strconv.Itoa(sequence))
}

func attributeKey(key string, value string, height types.Height, txIndex int, eventType string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("attr2/")
	buffer.WriteString(hex.EncodeToString([]byte(key)))
	buffer.WriteByte('/')
	buffer.WriteString(hex.EncodeToString([]byte(value)))
	buffer.WriteByte('/')
	buffer.WriteString(strconv.FormatUint(uint64(height), 10))
	buffer.WriteByte('/')
	buffer.WriteString(strconv.Itoa(txIndex))
	buffer.WriteByte('/')
	buffer.WriteString(hex.EncodeToString([]byte(eventType)))
	return buffer.Bytes()
}

func attributePrefix(key string, value string) []byte {
	return []byte("attr2/" + hex.EncodeToString([]byte(key)) + "/" + hex.EncodeToString([]byte(value)) + "/")
}

func legacyAttributeKey(key string, value string, height types.Height, txIndex int, eventType string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("attr/")
	buffer.WriteString(key)
	buffer.WriteByte('/')
	buffer.WriteString(value)
	buffer.WriteByte('/')
	buffer.WriteString(strconv.FormatUint(uint64(height), 10))
	buffer.WriteByte('/')
	buffer.WriteString(strconv.Itoa(txIndex))
	buffer.WriteByte('/')
	buffer.WriteString(eventType)
	return buffer.Bytes()
}

func cloneEvent(event Event) Event {
	event.Attributes = append([]Attribute(nil), event.Attributes...)
	return event
}
