package ibc

import (
	"context"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
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
	root := strings.Repeat("02", 32)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:client-create:07-vexo-0:counterparty:5:" + hash),
		types.Tx("ibc:client-update:07-vexo-0:6:" + hash + ":" + root),
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
	if err != nil || !found || client.ChainID != "counterparty" || client.LatestHeight != 6 {
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

func TestModuleEmitsIndexedPacketEvents(t *testing.T) {
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7}
	result := types.Result{}
	tx := types.Tx("ibc:packet-send:2:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")
	emitted := module.Events(ctx, tx, result)
	if len(emitted) != 1 || emitted[0].Type != "ibc_packet-send" {
		t.Fatalf("unexpected events: %+v", emitted)
	}
	attributes := map[string]events.Attribute{}
	for _, attribute := range emitted[0].Attributes {
		attributes[attribute.Key] = attribute
	}
	for key, value := range map[string]string{
		"ibc_packet_event":        "send",
		"ibc_packet_id":           "transfer/channel-0/2",
		"ibc_sequence":            "2",
		"ibc_source_port":         "transfer",
		"ibc_source_channel":      "channel-0",
		"ibc_destination_port":    "transfer",
		"ibc_destination_channel": "channel-1",
		"ibc_data":                "cGF5bG9hZA",
		"ibc_timeout_height":      "10",
	} {
		if attributes[key].Value != value {
			t.Fatalf("expected %s=%s, got %+v", key, value, attributes[key])
		}
	}
	if !attributes["ibc_packet_event"].Index || !attributes["ibc_packet_id"].Index || attributes["ibc_data"].Index {
		t.Fatalf("unexpected index flags: %+v", attributes)
	}
}

func TestModuleTimeoutsPackets(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	hash := strings.Repeat("01", 32)
	sendCtx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:client-create:07-vexo-0:counterparty:5:" + hash),
		types.Tx("ibc:connection-open:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:channel-open:transfer:channel-0:connection-0:channel-1:ordered"),
	} {
		if result := module.DeliverTx(sendCtx, tx); result.Code != 0 {
			t.Fatalf("setup %q failed: %+v", tx, result)
		}
	}
	packetTx := types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")
	if result := module.DeliverTx(sendCtx, packetTx); result.Code != 0 {
		t.Fatalf("send failed: %+v", result)
	}
	earlyCtx := vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}
	if result := module.DeliverTx(earlyCtx, types.Tx("ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")); result.Code == 0 {
		t.Fatalf("expected early timeout failure")
	}
	timeoutCtx := vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}
	if result := module.DeliverTx(timeoutCtx, types.Tx("ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10")); result.Code != 0 {
		t.Fatalf("timeout failed: %+v", result)
	}
	keeper := ibckeeper.NewKeeper(storage)
	receipt, found, err := keeper.PacketReceipt(context.Background(), ibckeeper.Packet{
		Sequence:           1,
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

func TestModuleConnectionAndChannelHandshake(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:connection-open-try:connection-2:07-vexo-0:connection-3"),
		types.Tx("ibc:connection-open-confirm:connection-2"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
		types.Tx("ibc:channel-open-try:transfer:channel-2:connection-2:channel-3:unordered"),
		types.Tx("ibc:channel-open-confirm:transfer:channel-2"),
	} {
		if result := module.DeliverTx(ctx, tx); result.Code != 0 {
			t.Fatalf("deliver %q failed: %+v", tx, result)
		}
	}
	keeper := ibckeeper.NewKeeper(storage)
	connection, found, err := keeper.Connection(context.Background(), "connection-0")
	if err != nil || !found || connection.State != ibckeeper.StateOpen {
		t.Fatalf("unexpected connection found=%t connection=%+v err=%v", found, connection, err)
	}
	channel, found, err := keeper.Channel(context.Background(), "transfer", "channel-2")
	if err != nil || !found || channel.State != ibckeeper.StateOpen {
		t.Fatalf("unexpected channel found=%t channel=%+v err=%v", found, channel, err)
	}
	if result := module.DeliverTx(ctx, types.Tx("ibc:channel-open-confirm:transfer:channel-0")); result.Code == 0 {
		t.Fatalf("expected invalid confirm transition")
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
	gas, err = module.EstimateGas(vexoapp.Context{}, types.Tx("ibc:client-update:07-vexo-0:6:"+strings.Repeat("01", 32)+":"+strings.Repeat("02", 32)))
	if err != nil || gas != clientUpdateGasCost {
		t.Fatalf("unexpected client update gas %d err=%v", gas, err)
	}
	gas, err = module.EstimateGas(vexoapp.Context{}, types.Tx("ibc:connection-open-ack:connection-0"))
	if err != nil || gas != connectionAckGasCost {
		t.Fatalf("unexpected connection ack gas %d err=%v", gas, err)
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
	if err := command.Execute(&output, []string{"tx", "client-update", "07-vexo-0", "6", strings.Repeat("01", 32), strings.Repeat("02", 32)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:client-update:07-vexo-0:6:"+strings.Repeat("01", 32)+":"+strings.Repeat("02", 32)) {
		t.Fatalf("unexpected client update output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "connection-open-init", "connection-0", "07-vexo-0", "connection-1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:connection-open-init:connection-0:07-vexo-0:connection-1") {
		t.Fatalf("unexpected connection init output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "channel-open-ack", "transfer", "channel-0"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:channel-open-ack:transfer:channel-0") {
		t.Fatalf("unexpected channel ack output: %s", output.String())
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
