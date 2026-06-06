package signbytes

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestVoteSignBytesAreCanonicalAndStable(t *testing.T) {
	var hash types.Hash
	for index := range hash {
		hash[index] = byte(index + 1)
	}
	got := Vote(7, 3, hash)
	want := make([]byte, 0, len("vote")+8+8+len(hash))
	want = append(want, []byte("vote")...)
	want = binary.BigEndian.AppendUint64(want, 7)
	want = binary.BigEndian.AppendUint64(want, 3)
	want = append(want, hash[:]...)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected vote sign bytes\n got: %x\nwant: %x", got, want)
	}
	got[0] = 'x'
	if bytes.Equal(got, Vote(7, 3, hash)) {
		t.Fatalf("Vote returned aliased or mutable canonical bytes")
	}
}
