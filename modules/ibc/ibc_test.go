package ibc

import (
	"context"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestModuleStoresClientChannelAndPacket(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	hash := strings.Repeat("01", 32)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:client-create:07-vexo-0:counterparty:5:" + hash),
		types.Tx("ibc:connection-open:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:channel-open:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA"),
		types.Tx("ibc:packet-ack:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr"),
	} {
		if result := module.DeliverTx(ctx, tx); result.Code != 0 {
			t.Fatalf("deliver %q failed: %+v", tx, result)
		}
	}
	keeper := ibckeeper.NewKeeper(storage)
	client, found, err := keeper.Client(context.Background(), "07-vexo-0")
	if err != nil || !found || client.ChainID != "counterparty" {
		t.Fatalf("unexpected client found=%t client=%+v err=%v", found, client, err)
	}
	channel, found, err := keeper.Channel(context.Background(), "transfer", "channel-0")
	if err != nil || !found || channel.ConnectionID != "connection-0" {
		t.Fatalf("unexpected channel found=%t channel=%+v err=%v", found, channel, err)
	}
	receipt, found, err := keeper.PacketReceipt(context.Background(), ibckeeper.Packet{
		Sequence:           1,
		SourcePort:         "transfer",
		SourceChannel:      "channel-0",
		DestinationPort:    "transfer",
		DestinationChannel: "channel-1",
		Data:               []byte("payload"),
	})
	if err != nil || !found || receipt.CommitHeight != 7 || string(receipt.Packet.Data) != "payload" || !receipt.Acknowledged || string(receipt.Ack) != "ack" {
		t.Fatalf("unexpected receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
	query := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"packet", "1", "transfer", "channel-0", "transfer", "channel-1"}})
	if query.Code != 0 || !strings.Contains(string(query.Value), `"commit_height":7`) {
		t.Fatalf("unexpected query response: %+v", query)
	}
}

func TestModuleTimeoutsPackets(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	packetTx := types.Tx("ibc:packet-send:2:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")
	sendCtx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	if result := module.DeliverTx(sendCtx, packetTx); result.Code != 0 {
		t.Fatalf("send failed: %+v", result)
	}
	earlyCtx := vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}
	if result := module.DeliverTx(earlyCtx, types.Tx("ibc:packet-timeout:2:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")); result.Code == 0 {
		t.Fatalf("expected early timeout failure")
	}
	timeoutCtx := vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}
	if result := module.DeliverTx(timeoutCtx, types.Tx("ibc:packet-timeout:2:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")); result.Code != 0 {
		t.Fatalf("timeout failed: %+v", result)
	}
	keeper := ibckeeper.NewKeeper(storage)
	receipt, found, err := keeper.PacketReceipt(context.Background(), ibckeeper.Packet{
		Sequence:           2,
		SourcePort:         "transfer",
		SourceChannel:      "channel-0",
		DestinationPort:    "transfer",
		DestinationChannel: "channel-1",
		Data:               []byte("payload"),
		TimeoutHeight:      10,
	})
	if err != nil || !found || !receipt.TimedOut || receipt.TimeoutAt != 10 {
		t.Fatalf("unexpected timeout receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
}

func TestModuleIBCEventsAndGas(t *testing.T) {
	module := NewModule()
	gas, err := module.EstimateGas(vexoapp.Context{}, types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA"))
	if err != nil || gas != packetSendGasCost {
		t.Fatalf("unexpected gas %d err=%v", gas, err)
	}
	gas, err = module.EstimateGas(vexoapp.Context{}, types.Tx("ibc:packet-ack:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr"))
	if err != nil || gas != packetAckGasCost {
		t.Fatalf("unexpected ack gas %d err=%v", gas, err)
	}
	events := module.Events(vexoapp.Context{}, types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA"), types.Result{})
	if len(events) != 1 || events[0].Type != "ibc_packet-send" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestIBCModuleCLICommands(t *testing.T) {
	command := ibcCLICommand()
	var output strings.Builder
	if err := command.Execute(&output, []string{"tx", "packet-send", "1", "transfer", "channel-0", "transfer", "channel-1", "payload"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA") {
		t.Fatalf("unexpected tx output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "packet-ack", "1", "transfer", "channel-0", "transfer", "channel-1", "payload", "ack"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-ack:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr") {
		t.Fatalf("unexpected ack tx output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "packet-timeout", "1", "transfer", "channel-0", "transfer", "channel-1", "payload", "10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10") {
		t.Fatalf("unexpected timeout tx output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "packet", "1", "transfer", "channel-0", "transfer", "channel-1"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: ibc/packet/1/transfer/channel-0/transfer/channel-1" {
		t.Fatalf("unexpected query output: %s", output.String())
	}
}
