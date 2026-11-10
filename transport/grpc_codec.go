package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
	"google.golang.org/grpc/encoding"
)

const (
	grpcCodecName    = "vexo-binary"
	grpcCodecVersion = byte(3)

	// Handshake frames are exchanged before the peer is authenticated, so every
	// field is bounded far below the envelope size limit.
	maxHandshakeTextBytes      = 1024
	maxHandshakeAddressBytes   = 255
	maxHandshakeKeyBytes       = 4096
	maxHandshakeSignatureBytes = 4096
	maxHandshakeChallengeBytes = 64
	maxHandshakeProofBytes     = 256
	maxHandshakePeers          = 256
	maxEnvelopeTopicBytes      = 64
)

type grpcEnvelope struct {
	Topic p2p.Topic  `json:"topic"`
	From  p2p.PeerID `json:"from"`
	To    p2p.PeerID `json:"to,omitempty"`
	Data  []byte     `json:"data"`
}

// grpcStreamMessage is one frame of the Gossip stream. The stream opens with
// four handshake frames (hello, challenge, proof, accept) and then carries
// envelopes.
type grpcStreamMessage struct {
	Handshake *Handshake    `json:"handshake,omitempty"`
	Envelope  *grpcEnvelope `json:"envelope,omitempty"`
}

type grpcBinaryCodec struct{}

func init() {
	encoding.RegisterCodec(grpcBinaryCodec{})
}

func (grpcBinaryCodec) Name() string { return grpcCodecName }

func (grpcBinaryCodec) Marshal(value any) ([]byte, error) {
	switch message := value.(type) {
	case *grpcStreamMessage:
		return encodeGRPCStreamMessage(message)
	case grpcStreamMessage:
		return encodeGRPCStreamMessage(&message)
	default:
		return nil, fmt.Errorf("unsupported grpc codec value %T", value)
	}
}

func (grpcBinaryCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*grpcStreamMessage)
	if !ok {
		return fmt.Errorf("unsupported grpc codec target %T", value)
	}
	decoded, err := decodeGRPCStreamMessage(data)
	if err != nil {
		return err
	}
	*message = *decoded
	return nil
}

func encodeGRPCStreamMessage(message *grpcStreamMessage) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte(grpcCodecVersion)
	if message != nil && message.Handshake != nil {
		buffer.WriteByte(1)
		handshake := message.Handshake
		writeBinaryString(&buffer, handshake.ProtocolVersion)
		writeBinaryString(&buffer, handshake.NetworkID)
		writeBinaryString(&buffer, handshake.ChainID)
		writeBinaryString(&buffer, handshake.GenesisHash)
		writeBinaryString(&buffer, string(handshake.NodeID))
		writeBinaryString(&buffer, handshake.ListenAddr)
		writeBinaryBytes(&buffer, handshake.NodePublicKey)
		writeBinaryBytes(&buffer, handshake.Challenge)
		writeBinaryString(&buffer, handshake.AuthToken)
		writeBinaryBytes(&buffer, handshake.Signature)
		writeBinaryPeerMap(&buffer, handshake.KnownPeers)
	} else {
		buffer.WriteByte(0)
	}
	if message != nil && message.Envelope != nil {
		buffer.WriteByte(1)
		writeBinaryString(&buffer, string(message.Envelope.Topic))
		writeBinaryString(&buffer, string(message.Envelope.From))
		writeBinaryString(&buffer, string(message.Envelope.To))
		writeBinaryBytes(&buffer, message.Envelope.Data)
	} else {
		buffer.WriteByte(0)
	}
	return buffer.Bytes(), nil
}

func decodeGRPCStreamMessage(data []byte) (*grpcStreamMessage, error) {
	reader := bytes.NewReader(data)
	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != grpcCodecVersion {
		return nil, fmt.Errorf("unsupported grpc codec version %d", version)
	}
	message := &grpcStreamMessage{}
	hasHandshake, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch hasHandshake {
	case 0:
	case 1:
		message.Handshake, err = readHandshake(reader)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid handshake flag %d", hasHandshake)
	}
	hasEnvelope, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch hasEnvelope {
	case 0:
	case 1:
		topic, err := readBoundedString(reader, maxEnvelopeTopicBytes)
		if err != nil {
			return nil, err
		}
		from, err := readBoundedString(reader, p2p.MaxPeerIDBytes)
		if err != nil {
			return nil, err
		}
		to, err := readBoundedString(reader, p2p.MaxPeerIDBytes)
		if err != nil {
			return nil, err
		}
		payload, err := readBinaryBytes(reader)
		if err != nil {
			return nil, err
		}
		message.Envelope = &grpcEnvelope{
			Topic: p2p.Topic(topic),
			From:  p2p.PeerID(from),
			To:    p2p.PeerID(to),
			Data:  payload,
		}
	default:
		return nil, fmt.Errorf("invalid envelope flag %d", hasEnvelope)
	}
	if reader.Len() != 0 {
		return nil, errors.New("trailing grpc codec bytes")
	}
	return message, nil
}

func readHandshake(reader *bytes.Reader) (*Handshake, error) {
	var handshake Handshake
	var err error
	if handshake.ProtocolVersion, err = readBoundedString(reader, maxHandshakeTextBytes); err != nil {
		return nil, err
	}
	if handshake.NetworkID, err = readBoundedString(reader, maxHandshakeTextBytes); err != nil {
		return nil, err
	}
	if handshake.ChainID, err = readBoundedString(reader, maxHandshakeTextBytes); err != nil {
		return nil, err
	}
	if handshake.GenesisHash, err = readBoundedString(reader, maxHandshakeTextBytes); err != nil {
		return nil, err
	}
	nodeID, err := readBoundedString(reader, p2p.MaxPeerIDBytes)
	if err != nil {
		return nil, err
	}
	handshake.NodeID = p2p.PeerID(nodeID)
	if handshake.ListenAddr, err = readBoundedString(reader, maxHandshakeAddressBytes); err != nil {
		return nil, err
	}
	publicKey, err := readBoundedBytes(reader, maxHandshakeKeyBytes)
	if err != nil {
		return nil, err
	}
	handshake.NodePublicKey = types.PublicKey(publicKey)
	if handshake.Challenge, err = readBoundedBytes(reader, maxHandshakeChallengeBytes); err != nil {
		return nil, err
	}
	if handshake.AuthToken, err = readBoundedString(reader, maxHandshakeProofBytes); err != nil {
		return nil, err
	}
	signature, err := readBoundedBytes(reader, maxHandshakeSignatureBytes)
	if err != nil {
		return nil, err
	}
	handshake.Signature = types.Signature(signature)
	if handshake.KnownPeers, err = readBinaryPeerMap(reader); err != nil {
		return nil, err
	}
	return &handshake, nil
}

func writeBinaryString(writer io.Writer, value string) {
	writeBinaryBytes(writer, []byte(value))
}

func writeBinaryBytes(writer io.Writer, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func writeBinaryPeerMap(writer io.Writer, peers map[p2p.PeerID]string) {
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(peers)))
	_, _ = writer.Write(count[:])
	for _, peerID := range sortedPeerIDs(peers) {
		writeBinaryString(writer, string(peerID))
		writeBinaryString(writer, peers[peerID])
	}
}

func sortedPeerIDs(peers map[p2p.PeerID]string) []p2p.PeerID {
	peerIDs := make([]p2p.PeerID, 0, len(peers))
	for peerID := range peers {
		peerIDs = append(peerIDs, peerID)
	}
	sort.Slice(peerIDs, func(left int, right int) bool { return peerIDs[left] < peerIDs[right] })
	return peerIDs
}

func readBinaryString(reader io.Reader) (string, error) {
	value, err := readBinaryBytes(reader)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func readBoundedString(reader io.Reader, limit uint32) (string, error) {
	value, err := readBoundedBytes(reader, limit)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// readBinaryPeerMap decodes a gossiped peer list. The list is an unsigned hint
// from an untrusted source, so it is strictly bounded and any malformed entry
// rejects the whole frame instead of being silently dropped: sender and
// receiver must agree on exactly what was signed.
func readBinaryPeerMap(reader io.Reader) (map[p2p.PeerID]string, error) {
	var countBytes [4]byte
	if _, err := io.ReadFull(reader, countBytes[:]); err != nil {
		return nil, err
	}
	count := binary.BigEndian.Uint32(countBytes[:])
	if count > maxHandshakePeers {
		return nil, ErrMessageTooLarge
	}
	peers := make(map[p2p.PeerID]string, int(count))
	for index := uint32(0); index < count; index++ {
		peerID, err := readBoundedString(reader, p2p.MaxPeerIDBytes)
		if err != nil {
			return nil, err
		}
		address, err := readBoundedString(reader, maxHandshakeAddressBytes)
		if err != nil {
			return nil, err
		}
		if !p2p.ValidPeerID(p2p.PeerID(peerID)) || !p2p.ValidPeerAddress(address) {
			return nil, fmt.Errorf("%w: invalid known peer entry", ErrHandshakeFailed)
		}
		if _, duplicate := peers[p2p.PeerID(peerID)]; duplicate {
			return nil, fmt.Errorf("%w: duplicate known peer entry", ErrHandshakeFailed)
		}
		peers[p2p.PeerID(peerID)] = address
	}
	return peers, nil
}

func readBinaryBytes(reader io.Reader) ([]byte, error) {
	return readBoundedBytes(reader, MinMessageBytes)
}

func readBoundedBytes(reader io.Reader, limit uint32) ([]byte, error) {
	var lengthBytes [4]byte
	if _, err := io.ReadFull(reader, lengthBytes[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBytes[:])
	if length > limit {
		return nil, ErrMessageTooLarge
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, err
	}
	return value, nil
}
