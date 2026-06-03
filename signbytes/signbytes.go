package signbytes

import (
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

func Vote(height types.Height, round types.Round, blockHash types.Hash) []byte {
	message := make([]byte, 0, len(blockHash)+20)
	message = append(message, []byte("vote")...)
	message = binary.BigEndian.AppendUint64(message, uint64(height))
	message = binary.BigEndian.AppendUint64(message, uint64(round))
	message = append(message, blockHash[:]...)
	return message
}
