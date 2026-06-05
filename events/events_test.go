package events

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
)

func TestIndexerIndexesAndQueriesAttributes(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	indexer := NewIndexer(storage)
	err = indexer.IndexBlock(context.Background(), 7, [][]Event{
		{
			{Type: "transfer", Attributes: []Attribute{
				{Key: "sender", Value: "alice", Index: true},
				{Key: "recipient", Value: "bob", Index: true},
			}},
		},
		{
			{Type: "stake", Attributes: []Attribute{{Key: "validator", Value: "validator-1", Index: true}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err := indexer.Query(context.Background(), "sender", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Height != 7 || records[0].Event.Type != "transfer" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func TestIndexerUsesEscapedAttributePrefix(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	indexer := NewIndexer(storage)
	if err := indexer.IndexBlock(context.Background(), 9, [][]Event{
		{
			{Type: "one", Attributes: []Attribute{{Key: "a", Value: "b/c", Index: true}}},
			{Type: "two", Attributes: []Attribute{{Key: "a/b", Value: "c", Index: true}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	records, err := indexer.Query(context.Background(), "a", "b/c")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Event.Type != "one" {
		t.Fatalf("expected exact escaped match, got %+v", records)
	}
	records, err = indexer.Query(context.Background(), "a/b", "c")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Event.Type != "two" {
		t.Fatalf("expected exact escaped match for second key, got %+v", records)
	}
}
