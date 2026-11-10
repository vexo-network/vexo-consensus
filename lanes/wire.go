package lanes

import (
	"encoding/binary"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

// ErrInvalidMessage reports a lane message that is not well formed.
var ErrInvalidMessage = errors.New("invalid lane message")

// MaxRequestIDs bounds the batches one request asks for.
const MaxRequestIDs = 256

// MessageType names the kind of a lane message.
type MessageType byte

const (
	// MessageBatch carries a batch: its author disseminating it, or a validator serving a request.
	MessageBatch MessageType = 1
	// MessageAck acknowledges to the author that a validator stores its batch.
	MessageAck MessageType = 2
	// MessageCert announces that a batch is certified available.
	MessageCert MessageType = 3
	// MessageRequest asks a validator for batches it stores.
	MessageRequest MessageType = 4
)

// wireVersion leads every lane message.
const wireVersion = 1

// Request asks for the batches of IDs.
type Request struct {
	IDs []types.Hash
}

// Message is one lane message on p2p.TopicBatch.
type Message struct {
	Type    MessageType
	Batch   *Batch
	Ack     *Ack
	Cert    *Cert
	Request *Request
}

// encode encodes message in the binary lane format: a version byte, a type byte, and the body as
// uvarint-length-prefixed byte strings and fixed-size hashes. Batches are the bulk of what
// validators exchange, so they carry their transactions as raw bytes.
func encode(message Message) ([]byte, error) {
	buffer := []byte{wireVersion, byte(message.Type)}
	switch {
	case message.Type == MessageBatch && message.Batch != nil:
		batch := message.Batch
		size := 64 + len(batch.ChainID) + len(batch.Author) + len(batch.Signature) + batch.Size() + 4*len(batch.Txs)
		buffer = append(make([]byte, 0, size), buffer...)
		buffer = appendBytes(buffer, []byte(batch.ChainID))
		buffer = appendBytes(buffer, []byte(batch.Author))
		buffer = binary.AppendUvarint(buffer, batch.Seq)
		buffer = binary.AppendUvarint(buffer, uint64(len(batch.Txs)))
		for _, tx := range batch.Txs {
			buffer = appendBytes(buffer, tx)
		}
		buffer = appendBytes(buffer, batch.Signature)
	case message.Type == MessageAck && message.Ack != nil:
		buffer = append(buffer, message.Ack.BatchID[:]...)
		buffer = appendBytes(buffer, []byte(message.Ack.Voter))
		buffer = appendBytes(buffer, message.Ack.Signature)
	case message.Type == MessageCert && message.Cert != nil:
		buffer = append(buffer, message.Cert.BatchID[:]...)
		buffer = appendBytes(buffer, []byte(message.Cert.Author))
		buffer = appendBytes(buffer, message.Cert.Signers)
		buffer = appendBytes(buffer, message.Cert.Signature)
	case message.Type == MessageRequest && message.Request != nil:
		buffer = binary.AppendUvarint(buffer, uint64(len(message.Request.IDs)))
		for _, id := range message.Request.IDs {
			buffer = append(buffer, id[:]...)
		}
	default:
		return nil, ErrInvalidMessage
	}
	return buffer, nil
}

// Decode decodes a lane message. It refuses a message that does not parse exactly, with trailing
// bytes, or over the limits.
func Decode(data []byte) (Message, error) {
	reader := wireReader{data: data}
	if reader.byte() != wireVersion {
		return Message{}, ErrInvalidMessage
	}
	message := Message{Type: MessageType(reader.byte())}
	switch message.Type {
	case MessageBatch:
		batch := Batch{ChainID: string(reader.bytes(256)), Author: types.ValidatorID(reader.bytes(256)), Seq: reader.uvarint()}
		count := reader.uvarint()
		if count > MaxBatchTxs {
			return Message{}, ErrInvalidMessage
		}
		batch.Txs = make([]types.Tx, 0, count)
		for index := uint64(0); index < count && reader.err == nil; index++ {
			batch.Txs = append(batch.Txs, types.Tx(reader.bytes(MaxBatchBytes)))
		}
		batch.Signature = reader.bytes(1024)
		message.Batch = &batch
	case MessageAck:
		ack := Ack{BatchID: reader.hash(), Voter: types.ValidatorID(reader.bytes(256)), Signature: reader.bytes(1024)}
		message.Ack = &ack
	case MessageCert:
		cert := Cert{BatchID: reader.hash(), Author: types.ValidatorID(reader.bytes(256)), Signers: reader.bytes(1 << 16), Signature: reader.bytes(1 << 20)}
		message.Cert = &cert
	case MessageRequest:
		count := reader.uvarint()
		if count == 0 || count > MaxRequestIDs {
			return Message{}, ErrInvalidMessage
		}
		request := Request{IDs: make([]types.Hash, 0, count)}
		for index := uint64(0); index < count && reader.err == nil; index++ {
			request.IDs = append(request.IDs, reader.hash())
		}
		message.Request = &request
	default:
		return Message{}, ErrInvalidMessage
	}
	if reader.err != nil || len(reader.data) != 0 {
		return Message{}, ErrInvalidMessage
	}
	return message, nil
}

func appendBytes(buffer []byte, value []byte) []byte {
	buffer = binary.AppendUvarint(buffer, uint64(len(value)))
	return append(buffer, value...)
}

// wireReader reads a lane message; the first failure sticks.
type wireReader struct {
	data []byte
	err  error
}

func (reader *wireReader) fail() {
	reader.err = ErrInvalidMessage
	reader.data = nil
}

func (reader *wireReader) byte() byte {
	if reader.err != nil || len(reader.data) == 0 {
		reader.fail()
		return 0
	}
	value := reader.data[0]
	reader.data = reader.data[1:]
	return value
}

func (reader *wireReader) uvarint() uint64 {
	if reader.err != nil {
		return 0
	}
	value, read := binary.Uvarint(reader.data)
	if read <= 0 {
		reader.fail()
		return 0
	}
	reader.data = reader.data[read:]
	return value
}

func (reader *wireReader) bytes(limit uint64) []byte {
	length := reader.uvarint()
	if reader.err != nil || length > limit || length > uint64(len(reader.data)) {
		reader.fail()
		return nil
	}
	value := append([]byte(nil), reader.data[:length]...)
	reader.data = reader.data[length:]
	return value
}

func (reader *wireReader) hash() types.Hash {
	var hash types.Hash
	if reader.err != nil || len(reader.data) < len(hash) {
		reader.fail()
		return hash
	}
	copy(hash[:], reader.data)
	reader.data = reader.data[len(hash):]
	return hash
}
