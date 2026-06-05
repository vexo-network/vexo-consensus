package ibc

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func setupOpenIBCPath(t *testing.T, ctx context.Context, keeper *Keeper, latestHeight types.Height) ClientState {
	t.Helper()
	client := ClientState{
		ClientID:         "07-vexo-0",
		ChainID:          "counterparty",
		LatestHeight:     latestHeight,
		ValidatorSetHash: types.Hash{1},
	}
	if err := keeper.SetClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SetConnection(ctx, ConnectionState{ConnectionID: "connection-0", ClientID: client.ClientID, Counterparty: "connection-1", State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SetChannel(ctx, ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: "ordered", State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestKeeperPacketLifecycle(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()
	client := setupOpenIBCPath(t, ctx, keeper, 10)
	loaded, found, err := keeper.Client(ctx, client.ClientID)
	if err != nil || !found || loaded.ChainID != "counterparty" {
		t.Fatalf("unexpected client found=%t client=%+v err=%v", found, loaded, err)
	}
	packet := Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload"), TimeoutHeight: 13}
	if err := keeper.SendPacket(ctx, 11, packet); err != nil {
		t.Fatal(err)
	}
	if err := keeper.AcknowledgePacket(ctx, 12, packet, []byte("ack")); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := keeper.PacketReceipt(ctx, packet)
	if err != nil || !found || !receipt.Acknowledged || string(receipt.Ack) != "ack" || receipt.AckHeight != 12 {
		t.Fatalf("unexpected receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
	if err := keeper.TimeoutPacket(ctx, 13, packet); !errors.Is(err, ErrPacketAcked) {
		t.Fatalf("expected acked packet timeout rejection, got %v", err)
	}
}

func TestKeeperUpdatesClientAndVerifiesProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	keeper := NewKeeper(storage)
	client := ClientState{
		ClientID:         "07-vexo-0",
		ChainID:          "counterparty",
		LatestHeight:     10,
		ValidatorSetHash: types.Hash{1},
	}
	if err := keeper.SetClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(ctx, "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.Build(ctx, storage, "counterparty", 11, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.UpdateClient(ctx, client.ClientID, 11, types.Hash{2}, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
	updated, found, err := keeper.Client(ctx, client.ClientID)
	if err != nil || !found || updated.LatestHeight != 11 || updated.LatestStateRoot != proof.StateRoot {
		t.Fatalf("unexpected updated client found=%t client=%+v err=%v", found, updated, err)
	}
	if err := keeper.UpdateClient(ctx, client.ClientID, 10, types.Hash{3}, proof.StateRoot); !errors.Is(err, ErrStaleClientUpdate) {
		t.Fatalf("expected stale update rejection, got %v", err)
	}
	if err := keeper.VerifyClientProof(ctx, client.ClientID, proof); err != nil {
		t.Fatal(err)
	}
	proof.Value = []byte("200")
	if err := keeper.VerifyClientProof(ctx, client.ClientID, proof); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("expected invalid proof rejection, got %v", err)
	}
}

func TestKeeperVerifiesPacketCommitmentProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	keeper := NewKeeper(storage)
	client := setupOpenIBCPath(t, ctx, keeper, 11)
	packet := Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload")}
	if err := keeper.SendPacket(ctx, 11, packet); err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.Build(ctx, storage, "counterparty", 11, Namespace, packetCommitmentKey(packet))
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.UpdateClient(ctx, client.ClientID, 11, types.Hash{2}, proof.StateRoot); !errors.Is(err, ErrStaleClientUpdate) {
		t.Fatalf("expected stale update because client is already at height 11, got %v", err)
	}
	client.LatestStateRoot = proof.StateRoot
	if err := keeper.SetClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := keeper.VerifyPacketCommitmentProof(ctx, client.ClientID, packet, proof); err != nil {
		t.Fatal(err)
	}
	wrongPacket := packet
	wrongPacket.Sequence = 2
	if err := keeper.VerifyPacketCommitmentProof(ctx, client.ClientID, wrongPacket, proof); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("expected wrong packet proof rejection, got %v", err)
	}
	proof.Value = []byte(`{"packet":{"sequence":1},"commit_height":11}`)
	if err := keeper.VerifyPacketCommitmentProof(ctx, client.ClientID, packet, proof); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("expected tampered packet proof rejection, got %v", err)
	}
}

func TestKeeperConnectionAndChannelHandshakeTransitions(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()
	if err := keeper.SetConnection(ctx, ConnectionState{ConnectionID: "connection-0", ClientID: "07-vexo-0", Counterparty: "connection-1", State: StateInit}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.UpdateConnectionState(ctx, "connection-0", StateTryOpen, StateOpen); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid connection transition, got %v", err)
	}
	if err := keeper.UpdateConnectionState(ctx, "connection-0", StateInit, StateOpen); err != nil {
		t.Fatal(err)
	}
	connection, found, err := keeper.Connection(ctx, "connection-0")
	if err != nil || !found || connection.State != StateOpen {
		t.Fatalf("unexpected connection found=%t connection=%+v err=%v", found, connection, err)
	}
	if err := keeper.SetChannel(ctx, ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: "ordered", State: StateTryOpen}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.UpdateChannelState(ctx, "transfer", "channel-0", StateInit, StateOpen); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid channel transition, got %v", err)
	}
	if err := keeper.UpdateChannelState(ctx, "transfer", "channel-0", StateTryOpen, StateOpen); err != nil {
		t.Fatal(err)
	}
	channel, found, err := keeper.Channel(ctx, "transfer", "channel-0")
	if err != nil || !found || channel.State != StateOpen {
		t.Fatalf("unexpected channel found=%t channel=%+v err=%v", found, channel, err)
	}
}

func TestKeeperPacketTimeoutLifecycle(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()
	setupOpenIBCPath(t, ctx, keeper, 10)
	packet := Packet{
		Sequence:           1,
		SourcePort:         "transfer",
		SourceChannel:      "channel-0",
		DestinationPort:    "transfer",
		DestinationChannel: "channel-1",
		Data:               []byte("payload"),
		TimeoutHeight:      20,
	}
	if err := keeper.SendPacket(ctx, 11, packet); err != nil {
		t.Fatal(err)
	}
	if err := keeper.TimeoutPacket(ctx, 19, packet); !errors.Is(err, ErrPacketNotTimedOut) {
		t.Fatalf("expected not-timed-out error, got %v", err)
	}
	if err := keeper.TimeoutPacket(ctx, 20, packet); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := keeper.PacketReceipt(ctx, packet)
	if err != nil || !found || !receipt.TimedOut || receipt.TimeoutAt != 20 {
		t.Fatalf("unexpected timeout receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
	if err := keeper.AcknowledgePacket(ctx, 21, packet, []byte("ack")); !errors.Is(err, ErrPacketTimedOut) {
		t.Fatalf("expected timed-out ack rejection, got %v", err)
	}
}

func TestKeeperRejectsInvalidPacketPathAndSequence(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()
	packet := Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload")}
	if err := keeper.SendPacket(ctx, 11, packet); !errors.Is(err, ErrInvalidChannel) {
		t.Fatalf("expected missing channel rejection, got %v", err)
	}
	client := ClientState{ClientID: "07-vexo-0", ChainID: "counterparty", LatestHeight: 10, ValidatorSetHash: types.Hash{1}}
	if err := keeper.SetClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SetConnection(ctx, ConnectionState{ConnectionID: "connection-0", ClientID: client.ClientID, Counterparty: "connection-1", State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SetChannel(ctx, ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: "ordered", State: StateInit}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SendPacket(ctx, 11, packet); !errors.Is(err, ErrChannelNotOpen) {
		t.Fatalf("expected closed channel rejection, got %v", err)
	}
	if err := keeper.UpdateChannelState(ctx, "transfer", "channel-0", StateInit, StateOpen); err != nil {
		t.Fatal(err)
	}
	gapPacket := packet
	gapPacket.Sequence = 2
	if err := keeper.SendPacket(ctx, 11, gapPacket); !errors.Is(err, ErrUnexpectedPacketSequence) {
		t.Fatalf("expected sequence gap rejection, got %v", err)
	}
	if err := keeper.SendPacket(ctx, 11, packet); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SendPacket(ctx, 11, packet); !errors.Is(err, ErrPacketAlreadyExists) {
		t.Fatalf("expected duplicate packet rejection, got %v", err)
	}
}
