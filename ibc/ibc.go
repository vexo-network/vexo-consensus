package ibc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const Namespace = "ibc"

const (
	StateInit    = "init"
	StateTryOpen = "tryopen"
	StateOpen    = "open"
)

const (
	OrderingOrdered   = "ordered"
	OrderingUnordered = "unordered"
)

var (
	ErrInvalidClient            = errors.New("invalid IBC client")
	ErrInvalidConnection        = errors.New("invalid IBC connection")
	ErrInvalidChannel           = errors.New("invalid IBC channel")
	ErrInvalidTransition        = errors.New("invalid IBC state transition")
	ErrInvalidPacket            = errors.New("invalid IBC packet")
	ErrInvalidAck               = errors.New("invalid IBC acknowledgement")
	ErrInvalidProof             = errors.New("invalid IBC proof")
	ErrClientNotFound           = errors.New("IBC client not found")
	ErrClientFrozen             = errors.New("IBC client is frozen")
	ErrClientExpired            = errors.New("IBC client trusting period expired")
	ErrStaleClientUpdate        = errors.New("stale IBC client update")
	ErrChannelNotOpen           = errors.New("IBC channel is not open")
	ErrConnectionNotOpen        = errors.New("IBC connection is not open")
	ErrUnexpectedPacketSequence = errors.New("unexpected IBC packet sequence")
	ErrPacketAlreadyExists      = errors.New("IBC packet commitment already exists")
	ErrPacketAcked              = errors.New("IBC packet already acknowledged")
	ErrPacketTimedOut           = errors.New("IBC packet already timed out")
	ErrPacketNotTimedOut        = errors.New("IBC packet timeout height has not elapsed")
	ErrStoreMissing             = errors.New("IBC store is required")
)

type ClientState struct {
	ClientID             string       `json:"client_id"`
	ChainID              string       `json:"chain_id"`
	LatestHeight         types.Height `json:"latest_height"`
	ValidatorSetHash     types.Hash   `json:"validator_set_hash"`
	LatestStateRoot      types.Hash   `json:"latest_state_root,omitempty"`
	Frozen               bool         `json:"frozen,omitempty"`
	TrustingPeriodHeight uint64       `json:"trusting_period_height,omitempty"`
}

type ConnectionState struct {
	ConnectionID string `json:"connection_id"`
	ClientID     string `json:"client_id"`
	Counterparty string `json:"counterparty"`
	State        string `json:"state"`
}

type ChannelState struct {
	PortID       string `json:"port_id"`
	ChannelID    string `json:"channel_id"`
	ConnectionID string `json:"connection_id"`
	Counterparty string `json:"counterparty"`
	Ordering     string `json:"ordering"`
	State        string `json:"state"`
}

type Packet struct {
	Sequence           uint64 `json:"sequence"`
	SourcePort         string `json:"source_port"`
	SourceChannel      string `json:"source_channel"`
	DestinationPort    string `json:"destination_port"`
	DestinationChannel string `json:"destination_channel"`
	Data               []byte `json:"data"`
	TimeoutHeight      uint64 `json:"timeout_height,omitempty"`
}

type PacketReceipt struct {
	Packet       Packet       `json:"packet"`
	CommitHeight types.Height `json:"commit_height"`
	Acknowledged bool         `json:"acknowledged,omitempty"`
	Ack          []byte       `json:"ack,omitempty"`
	AckHeight    types.Height `json:"ack_height,omitempty"`
	TimedOut     bool         `json:"timed_out,omitempty"`
	TimeoutAt    types.Height `json:"timeout_at,omitempty"`
}

type KVStore interface {
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
}

type Keeper struct {
	store KVStore
}

func NewKeeper(store KVStore) *Keeper {
	return &Keeper{store: store}
}

func (keeper *Keeper) SetClient(ctx context.Context, client ClientState) error {
	if err := validateClient(client); err != nil {
		return err
	}
	return keeper.setJSON(ctx, clientKey(client.ClientID), client)
}

func (keeper *Keeper) Client(ctx context.Context, clientID string) (ClientState, bool, error) {
	var client ClientState
	found, err := keeper.getJSON(ctx, clientKey(clientID), &client)
	return client, found, err
}

func (keeper *Keeper) UpdateClient(ctx context.Context, clientID string, latestHeight types.Height, validatorSetHash types.Hash, latestStateRoot types.Hash) error {
	if clientID == "" || latestHeight == 0 || validatorSetHash == (types.Hash{}) || latestStateRoot == (types.Hash{}) {
		return ErrInvalidClient
	}
	client, found, err := keeper.Client(ctx, clientID)
	if err != nil {
		return err
	}
	if !found {
		return ErrClientNotFound
	}
	if client.Frozen {
		return ErrClientFrozen
	}
	if latestHeight <= client.LatestHeight {
		return ErrStaleClientUpdate
	}
	client.LatestHeight = latestHeight
	client.ValidatorSetHash = validatorSetHash
	client.LatestStateRoot = latestStateRoot
	return keeper.SetClient(ctx, client)
}

func (keeper *Keeper) FreezeClient(ctx context.Context, clientID string) error {
	if clientID == "" {
		return ErrInvalidClient
	}
	client, found, err := keeper.Client(ctx, clientID)
	if err != nil {
		return err
	}
	if !found {
		return ErrClientNotFound
	}
	client.Frozen = true
	return keeper.SetClient(ctx, client)
}

func (keeper *Keeper) ClientExpired(ctx context.Context, clientID string, currentHeight types.Height) (bool, error) {
	client, found, err := keeper.Client(ctx, clientID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, ErrClientNotFound
	}
	return clientExpired(client, currentHeight), nil
}

func (keeper *Keeper) SetConnection(ctx context.Context, connection ConnectionState) error {
	if connection.ConnectionID == "" || connection.ClientID == "" || connection.Counterparty == "" || !validHandshakeState(connection.State) {
		return ErrInvalidConnection
	}
	return keeper.setJSON(ctx, connectionKey(connection.ConnectionID), connection)
}

func (keeper *Keeper) Connection(ctx context.Context, connectionID string) (ConnectionState, bool, error) {
	var connection ConnectionState
	found, err := keeper.getJSON(ctx, connectionKey(connectionID), &connection)
	return connection, found, err
}

func (keeper *Keeper) UpdateConnectionState(ctx context.Context, connectionID string, expectedState string, nextState string) error {
	if connectionID == "" || !validHandshakeState(expectedState) || !validHandshakeState(nextState) {
		return ErrInvalidConnection
	}
	connection, found, err := keeper.Connection(ctx, connectionID)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrKeyNotFound
	}
	if connection.State != expectedState || !validTransition(expectedState, nextState) {
		return ErrInvalidTransition
	}
	connection.State = nextState
	return keeper.SetConnection(ctx, connection)
}

func (keeper *Keeper) SetChannel(ctx context.Context, channel ChannelState) error {
	if channel.PortID == "" || channel.ChannelID == "" || channel.ConnectionID == "" || channel.Counterparty == "" || !validChannelOrdering(channel.Ordering) || !validHandshakeState(channel.State) {
		return ErrInvalidChannel
	}
	return keeper.setJSON(ctx, channelKey(channel.PortID, channel.ChannelID), channel)
}

func (keeper *Keeper) Channel(ctx context.Context, portID string, channelID string) (ChannelState, bool, error) {
	var channel ChannelState
	found, err := keeper.getJSON(ctx, channelKey(portID, channelID), &channel)
	return channel, found, err
}

func (keeper *Keeper) UpdateChannelState(ctx context.Context, portID string, channelID string, expectedState string, nextState string) error {
	if portID == "" || channelID == "" || !validHandshakeState(expectedState) || !validHandshakeState(nextState) {
		return ErrInvalidChannel
	}
	channel, found, err := keeper.Channel(ctx, portID, channelID)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrKeyNotFound
	}
	if channel.State != expectedState || !validTransition(expectedState, nextState) {
		return ErrInvalidTransition
	}
	channel.State = nextState
	return keeper.SetChannel(ctx, channel)
}

func (keeper *Keeper) SendPacket(ctx context.Context, height types.Height, packet Packet) error {
	if err := validatePacket(packet); err != nil {
		return err
	}
	if err := keeper.validatePacketPath(ctx, packet); err != nil {
		return err
	}
	if _, found, err := keeper.PacketReceipt(ctx, packet); err != nil {
		return err
	} else if found {
		return ErrPacketAlreadyExists
	}
	expectedSequence, err := keeper.nextSequence(ctx, packet.SourcePort, packet.SourceChannel)
	if err != nil {
		return err
	}
	if packet.Sequence != expectedSequence {
		return ErrUnexpectedPacketSequence
	}
	receipt := PacketReceipt{Packet: clonePacket(packet), CommitHeight: height}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	writes := []kvbatch.KVWrite{
		{Namespace: Namespace, Key: packetCommitmentKey(packet), Value: encoded},
		{Namespace: Namespace, Key: nextSequenceKey(packet.SourcePort, packet.SourceChannel), Value: []byte(strconv.FormatUint(packet.Sequence+1, 10))},
	}
	if batch, ok := keeper.store.(kvbatch.BatchKVStore); ok {
		return batch.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := keeper.store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (keeper *Keeper) AcknowledgePacket(ctx context.Context, height types.Height, packet Packet, ack []byte) error {
	if err := validatePacket(packet); err != nil {
		return err
	}
	if err := keeper.validatePacketPath(ctx, packet); err != nil {
		return err
	}
	if len(ack) == 0 {
		return ErrInvalidAck
	}
	var receipt PacketReceipt
	found, err := keeper.getJSON(ctx, packetCommitmentKey(packet), &receipt)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrKeyNotFound
	}
	if receipt.TimedOut {
		return ErrPacketTimedOut
	}
	if receipt.Acknowledged {
		return ErrPacketAcked
	}
	receipt.Acknowledged = true
	receipt.Ack = append([]byte(nil), ack...)
	receipt.AckHeight = height
	return keeper.setJSON(ctx, packetCommitmentKey(packet), receipt)
}

func (keeper *Keeper) AcknowledgePacketWithProof(ctx context.Context, height types.Height, clientID string, packet Packet, proof queryproof.Proof, ack []byte) error {
	if clientID == "" {
		return ErrInvalidClient
	}
	if err := keeper.VerifyPacketAcknowledgementProof(ctx, clientID, packet, proof, ack); err != nil {
		return err
	}
	return keeper.AcknowledgePacket(ctx, height, packet, ack)
}

func (keeper *Keeper) TimeoutPacket(ctx context.Context, height types.Height, packet Packet) error {
	if err := validatePacket(packet); err != nil {
		return err
	}
	if err := keeper.validatePacketPath(ctx, packet); err != nil {
		return err
	}
	if packet.TimeoutHeight == 0 || uint64(height) < packet.TimeoutHeight {
		return ErrPacketNotTimedOut
	}
	var receipt PacketReceipt
	found, err := keeper.getJSON(ctx, packetCommitmentKey(packet), &receipt)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrKeyNotFound
	}
	if receipt.Acknowledged {
		return ErrPacketAcked
	}
	if receipt.TimedOut {
		return ErrPacketTimedOut
	}
	receipt.TimedOut = true
	receipt.TimeoutAt = height
	return keeper.setJSON(ctx, packetCommitmentKey(packet), receipt)
}

func (keeper *Keeper) TimeoutPacketWithProof(ctx context.Context, height types.Height, clientID string, packet Packet, proof queryproof.Proof) error {
	if clientID == "" {
		return ErrInvalidClient
	}
	if err := keeper.VerifyPacketTimeoutProof(ctx, clientID, packet, proof); err != nil {
		return err
	}
	return keeper.TimeoutPacket(ctx, height, packet)
}

func (keeper *Keeper) PacketReceipt(ctx context.Context, packet Packet) (PacketReceipt, bool, error) {
	var receipt PacketReceipt
	found, err := keeper.getJSON(ctx, packetCommitmentKey(packet), &receipt)
	return receipt, found, err
}

func (keeper *Keeper) VerifyClientProof(ctx context.Context, clientID string, proof queryproof.Proof) error {
	return keeper.VerifyClientProofAt(ctx, clientID, 0, proof)
}

func (keeper *Keeper) VerifyClientProofAt(ctx context.Context, clientID string, currentHeight types.Height, proof queryproof.Proof) error {
	client, found, err := keeper.Client(ctx, clientID)
	if err != nil {
		return err
	}
	if !found {
		return ErrClientNotFound
	}
	if client.Frozen {
		return ErrClientFrozen
	}
	if clientExpired(client, currentHeight) {
		return ErrClientExpired
	}
	if client.LatestStateRoot == (types.Hash{}) {
		return ErrInvalidProof
	}
	if proof.Height != client.LatestHeight {
		return ErrInvalidProof
	}
	if err := queryproof.Verify(proof, client.ChainID, client.LatestHeight, client.LatestStateRoot); err != nil {
		return errors.Join(ErrInvalidProof, err)
	}
	return nil
}

func (keeper *Keeper) VerifyPacketCommitmentProof(ctx context.Context, clientID string, packet Packet, proof queryproof.Proof) error {
	if err := validatePacket(packet); err != nil {
		return err
	}
	if err := keeper.VerifyClientProof(ctx, clientID, proof); err != nil {
		return err
	}
	if proof.Namespace != Namespace || !bytes.Equal(proof.Key, packetCommitmentKey(packet)) || !proof.Exists {
		return ErrInvalidProof
	}
	var receipt PacketReceipt
	if err := json.Unmarshal(proof.Value, &receipt); err != nil {
		return errors.Join(ErrInvalidProof, err)
	}
	if !samePacket(receipt.Packet, packet) || receipt.CommitHeight == 0 {
		return ErrInvalidProof
	}
	return nil
}

func (keeper *Keeper) VerifyPacketAcknowledgementProof(ctx context.Context, clientID string, packet Packet, proof queryproof.Proof, ack []byte) error {
	if len(ack) == 0 {
		return ErrInvalidAck
	}
	receipt, err := keeper.verifiedPacketReceiptProof(ctx, clientID, packet, proof)
	if err != nil {
		return err
	}
	if !receipt.Acknowledged || receipt.AckHeight == 0 || !bytes.Equal(receipt.Ack, ack) {
		return ErrInvalidProof
	}
	return nil
}

func (keeper *Keeper) VerifyPacketTimeoutProof(ctx context.Context, clientID string, packet Packet, proof queryproof.Proof) error {
	if err := validatePacket(packet); err != nil {
		return err
	}
	if err := keeper.VerifyClientProof(ctx, clientID, proof); err != nil {
		return err
	}
	if proof.Namespace != Namespace || !bytes.Equal(proof.Key, packetCommitmentKey(packet)) {
		return ErrInvalidProof
	}
	if !proof.Exists {
		return nil
	}
	var receipt PacketReceipt
	if err := json.Unmarshal(proof.Value, &receipt); err != nil {
		return errors.Join(ErrInvalidProof, err)
	}
	if !samePacket(receipt.Packet, packet) || receipt.Acknowledged {
		return ErrInvalidProof
	}
	return nil
}

func (keeper *Keeper) verifiedPacketReceiptProof(ctx context.Context, clientID string, packet Packet, proof queryproof.Proof) (PacketReceipt, error) {
	if err := validatePacket(packet); err != nil {
		return PacketReceipt{}, err
	}
	if err := keeper.VerifyClientProof(ctx, clientID, proof); err != nil {
		return PacketReceipt{}, err
	}
	if proof.Namespace != Namespace || !bytes.Equal(proof.Key, packetCommitmentKey(packet)) || !proof.Exists {
		return PacketReceipt{}, ErrInvalidProof
	}
	var receipt PacketReceipt
	if err := json.Unmarshal(proof.Value, &receipt); err != nil {
		return PacketReceipt{}, errors.Join(ErrInvalidProof, err)
	}
	if !samePacket(receipt.Packet, packet) || receipt.CommitHeight == 0 {
		return PacketReceipt{}, ErrInvalidProof
	}
	return receipt, nil
}

func (keeper *Keeper) validatePacketPath(ctx context.Context, packet Packet) error {
	channel, found, err := keeper.Channel(ctx, packet.SourcePort, packet.SourceChannel)
	if err != nil {
		return err
	}
	if !found {
		return ErrInvalidChannel
	}
	if channel.State != StateOpen {
		return ErrChannelNotOpen
	}
	if channel.Counterparty != packet.DestinationChannel {
		return ErrInvalidChannel
	}
	connection, found, err := keeper.Connection(ctx, channel.ConnectionID)
	if err != nil {
		return err
	}
	if !found {
		return ErrInvalidConnection
	}
	if connection.State != StateOpen {
		return ErrConnectionNotOpen
	}
	if _, found, err := keeper.Client(ctx, connection.ClientID); err != nil {
		return err
	} else if !found {
		return ErrClientNotFound
	}
	return nil
}

func (keeper *Keeper) nextSequence(ctx context.Context, portID string, channelID string) (uint64, error) {
	if keeper == nil || keeper.store == nil {
		return 0, ErrStoreMissing
	}
	encoded, err := keeper.store.Get(ctx, Namespace, nextSequenceKey(portID, channelID))
	if errors.Is(err, store.ErrKeyNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	sequence, err := strconv.ParseUint(string(encoded), 10, 64)
	if err != nil || sequence == 0 {
		return 0, ErrUnexpectedPacketSequence
	}
	return sequence, nil
}

func (keeper *Keeper) setJSON(ctx context.Context, key []byte, value any) error {
	if keeper == nil || keeper.store == nil {
		return ErrStoreMissing
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return keeper.store.Set(ctx, Namespace, key, encoded)
}

func (keeper *Keeper) getJSON(ctx context.Context, key []byte, value any) (bool, error) {
	if keeper == nil || keeper.store == nil {
		return false, ErrStoreMissing
	}
	encoded, err := keeper.store.Get(ctx, Namespace, key)
	if errors.Is(err, store.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(encoded, value)
}

func validateClient(client ClientState) error {
	if client.ClientID == "" || client.ChainID == "" || client.LatestHeight == 0 || client.ValidatorSetHash == (types.Hash{}) {
		return ErrInvalidClient
	}
	if client.TrustingPeriodHeight > 0 && uint64(client.LatestHeight) > ^uint64(0)-client.TrustingPeriodHeight {
		return ErrInvalidClient
	}
	return nil
}

func clientExpired(client ClientState, currentHeight types.Height) bool {
	if currentHeight == 0 || client.TrustingPeriodHeight == 0 {
		return false
	}
	return uint64(currentHeight) > uint64(client.LatestHeight)+client.TrustingPeriodHeight
}

func validatePacket(packet Packet) error {
	if packet.Sequence == 0 || packet.SourcePort == "" || packet.SourceChannel == "" || packet.DestinationPort == "" || packet.DestinationChannel == "" || len(packet.Data) == 0 {
		return ErrInvalidPacket
	}
	return nil
}

func validHandshakeState(state string) bool {
	switch state {
	case StateInit, StateTryOpen, StateOpen:
		return true
	default:
		return false
	}
}

func validChannelOrdering(ordering string) bool {
	switch ordering {
	case OrderingOrdered, OrderingUnordered:
		return true
	default:
		return false
	}
}

func validTransition(from string, to string) bool {
	switch from {
	case StateInit:
		return to == StateOpen
	case StateTryOpen:
		return to == StateOpen
	default:
		return false
	}
}

func ValidatePacket(packet Packet) error {
	return validatePacket(packet)
}

func clientKey(clientID string) []byte { return []byte("clients/" + clientID) }

func connectionKey(connectionID string) []byte { return []byte("connections/" + connectionID) }

func channelKey(portID string, channelID string) []byte {
	return []byte("channels/" + portID + "/" + channelID)
}

func nextSequenceKey(portID string, channelID string) []byte {
	return []byte("next-sequence/" + portID + "/" + channelID)
}

func packetCommitmentKey(packet Packet) []byte {
	return []byte("packets/" + packet.SourcePort + "/" + packet.SourceChannel + "/" + strconv.FormatUint(packet.Sequence, 10))
}

func PacketCommitmentKey(packet Packet) []byte {
	return append([]byte(nil), packetCommitmentKey(packet)...)
}

func clonePacket(packet Packet) Packet {
	packet.Data = append([]byte(nil), packet.Data...)
	return packet
}

func samePacket(left Packet, right Packet) bool {
	return left.Sequence == right.Sequence &&
		left.SourcePort == right.SourcePort &&
		left.SourceChannel == right.SourceChannel &&
		left.DestinationPort == right.DestinationPort &&
		left.DestinationChannel == right.DestinationChannel &&
		bytes.Equal(left.Data, right.Data) &&
		left.TimeoutHeight == right.TimeoutHeight
}
