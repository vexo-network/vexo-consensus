package kvbatch

import "testing"

func TestKVWritePreservesDeleteAndValueSemantics(t *testing.T) {
	set := KVWrite{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")}
	if set.Delete {
		t.Fatal("set write must not be marked as delete")
	}
	if set.Namespace != "bank" || string(set.Key) != "alice" || string(set.Value) != "100" {
		t.Fatalf("unexpected set write: %+v", set)
	}

	deleteWrite := KVWrite{Namespace: "bank", Key: []byte("alice"), Delete: true}
	if !deleteWrite.Delete || len(deleteWrite.Value) != 0 {
		t.Fatalf("unexpected delete write: %+v", deleteWrite)
	}
}
