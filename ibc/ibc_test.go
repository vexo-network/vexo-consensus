package ibc

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestKeeperPacketLifecycle(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()
	client := ClientState{
		ClientID:         "07-vexo-0",
		ChainID:          "counterparty",
		LatestHeight:     10,
		ValidatorSetHash: types.Hash{1},
	}
	if err := keeper.SetClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := keeper.Client(ctx, client.ClientID)
	if err != nil || !found || loaded.ChainID != "counterparty" {
		t.Fatalf("unexpected client found=%t client=%+v err=%v", found, loaded, err)
	}
	if err := keeper.SetConnection(ctx, ConnectionState{ConnectionID: "connection-0", ClientID: client.ClientID, Counterparty: "connection-1", State: "open"}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SetChannel(ctx, ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: "ordered", State: "open"}); err != nil {
		t.Fatal(err)
	}
	packet := Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload")}
	if err := keeper.SendPacket(ctx, 11, packet); err != nil {
		t.Fatal(err)
	}
	if err := keeper.AcknowledgePacket(ctx, packet, []byte("ack")); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := keeper.PacketReceipt(ctx, packet)
	if err != nil || !found || !receipt.Acknowledged || string(receipt.Ack) != "ack" {
		t.Fatalf("unexpected receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
}
