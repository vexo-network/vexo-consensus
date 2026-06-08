package ibc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/queryproof"
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
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
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

func TestModuleGenesisQueriesGasAndErrorEdges(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	client := ibckeeper.ClientState{ClientID: "07-vexo-0", ChainID: "counterparty", LatestHeight: 9, ValidatorSetHash: types.Hash{1}, LatestStateRoot: types.Hash{2}}
	connection := ibckeeper.ConnectionState{ConnectionID: "connection-0", ClientID: "07-vexo-0", Counterparty: "connection-1", State: ibckeeper.StateOpen}
	channel := ibckeeper.ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: "ordered", State: ibckeeper.StateOpen}
	genesis := vexoapp.GenesisState{}
	for key, value := range map[string]any{
		"ibc:client:07-vexo-0":           client,
		"ibc:connection:connection-0":    connection,
		"ibc:channel:transfer/channel-0": channel,
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		genesis[key] = encoded
	}
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}
	if err := module.InitGenesis(ctx, genesis); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string][]string{
		"client":     {"client", "07-vexo-0"},
		"connection": {"connection", "connection-0"},
		"channel":    {"channel", "transfer", "channel-0"},
	} {
		response := module.Query(ctx, vexoapp.QueryRequest{Path: path})
		if response.Code != 0 {
			t.Fatalf("expected %s query to succeed, got %+v", name, response)
		}
	}
	if response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"packet", "bad", "transfer", "channel-0", "transfer", "channel-1"}}); response.Code == 0 {
		t.Fatalf("expected invalid packet query failure")
	}
	if response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"client", "missing"}}); response.Code == 0 {
		t.Fatalf("expected missing client query failure")
	}
	if response := module.Query(vexoapp.Context{}, vexoapp.QueryRequest{Path: []string{"client", "07-vexo-0"}}); response.Code != 1 {
		t.Fatalf("expected missing store query failure, got %+v", response)
	}
	if err := module.InitGenesis(ctx, vexoapp.GenesisState{"ibc:client:bad": []byte("{")}); err == nil {
		t.Fatalf("expected invalid genesis JSON failure")
	}
	if err := module.BeginBlock(vexoapp.Context{Store: storage, Height: 11}, types.Header{}); err != nil {
		t.Fatal(err)
	}
	if err := module.BeginBlock(vexoapp.Context{Store: storage}, types.Header{}); err != nil {
		t.Fatal(err)
	}
	if err := module.EndBlock(vexoapp.Context{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		tx       types.Tx
		expected uint64
	}{
		{types.Tx("ibc:client-create:07-vexo-1:counterparty:5:" + strings.Repeat("01", 32)), clientCreateGasCost},
		{types.Tx("ibc:connection-open-init:connection-1:07-vexo-0:connection-2"), connectionOpenGasCost},
		{types.Tx("ibc:channel-open-init:transfer:channel-1:connection-0:channel-2:ordered"), channelOpenGasCost},
		{types.Tx("ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10"), packetTimeoutGasCost},
		{types.Tx("ibc:packet-ack-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr:c:p"), packetAckGasCost},
		{types.Tx("ibc:packet-timeout-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10:c:p"), packetTimeoutGasCost},
	} {
		gas, err := module.EstimateGas(vexoapp.Context{}, tc.tx)
		if err != nil || gas != tc.expected {
			t.Fatalf("unexpected gas for %q: %d err=%v", tc.tx, gas, err)
		}
	}
	if _, err := module.EstimateGas(vexoapp.Context{}, types.Tx("ibc:bad")); err != ErrInvalidIBCTx {
		t.Fatalf("expected invalid gas estimate, got %v", err)
	}
	if events := module.Events(ctx, types.Tx("ibc:packet-ack-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr:c:p"), types.Result{}); len(events) != 1 || events[0].Type != "ibc_packet-ack-proof" {
		t.Fatalf("unexpected ack proof events: %+v", events)
	}
	if events := module.Events(ctx, types.Tx("bad"), types.Result{}); events != nil {
		t.Fatalf("expected invalid event tx to be ignored, got %+v", events)
	}
}

func TestModuleSDKWrappers(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	packet := ibckeeper.Packet{Sequence: 7, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload"), TimeoutHeight: 10}
	if NewKeeper(storage) == nil {
		t.Fatal("expected keeper wrapper")
	}
	hash := types.Hash{1}
	root := types.Hash{2}
	if err := ibckeeper.NewKeeper(storage).SetClient(ctx, ibckeeper.ClientState{ClientID: "07-vexo-0", ChainID: "counterparty", LatestHeight: 5, ValidatorSetHash: hash}); err != nil {
		t.Fatal(err)
	}
	if err := ibckeeper.NewKeeper(storage).SetConnection(ctx, ibckeeper.ConnectionState{ConnectionID: "connection-0", ClientID: "07-vexo-0", Counterparty: "connection-1", State: ibckeeper.StateOpen}); err != nil {
		t.Fatal(err)
	}
	if err := ibckeeper.NewKeeper(storage).SetChannel(ctx, ibckeeper.ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: ibckeeper.OrderingOrdered, State: ibckeeper.StateOpen}); err != nil {
		t.Fatal(err)
	}
	packet.Sequence = 1
	if SendPacket(ctx, storage, 5, packet) != nil {
		t.Fatal("expected send wrapper to store packet")
	}
	if err := AcknowledgePacket(ctx, storage, 6, packet, []byte("ack")); err != nil {
		t.Fatal(err)
	}
	ackReceipt, found, err := ibckeeper.NewKeeper(storage).PacketReceipt(ctx, packet)
	if err != nil || !found || !ackReceipt.Acknowledged {
		t.Fatalf("expected acked packet found=%t receipt=%+v err=%v", found, ackReceipt, err)
	}
	timeoutPacket := ibckeeper.Packet{Sequence: 2, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload"), TimeoutHeight: 10}
	if err := SendPacket(ctx, storage, 5, timeoutPacket); err != nil {
		t.Fatal(err)
	}
	if err := TimeoutPacket(ctx, storage, 10, timeoutPacket); err != nil {
		t.Fatal(err)
	}
	if path := PacketQueryPath(timeoutPacket); path != "ibc/packet/2/transfer/channel-0/transfer/channel-1" {
		t.Fatalf("unexpected packet query path %q", path)
	}
	if err := UpdateClient(ctx, storage, "07-vexo-0", 6, hash, root); err != nil {
		t.Fatal(err)
	}
	proof := queryproof.Proof{ChainID: "counterparty", Height: 6, StateRoot: root}
	if err := VerifyClientProof(ctx, storage, "07-vexo-0", proof); err == nil {
		t.Fatalf("expected incomplete proof to fail verification")
	}
	if err := ibckeeper.NewKeeper(storage).SetConnection(ctx, ibckeeper.ConnectionState{ConnectionID: "connection-0", ClientID: "07-vexo-0", Counterparty: "connection-1", State: ibckeeper.StateInit}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateConnectionState(ctx, storage, "connection-0", ibckeeper.StateInit, ibckeeper.StateOpen); err != nil {
		t.Fatal(err)
	}
	if err := ibckeeper.NewKeeper(storage).SetChannel(ctx, ibckeeper.ChannelState{PortID: "transfer", ChannelID: "channel-0", ConnectionID: "connection-0", Counterparty: "channel-1", Ordering: ibckeeper.OrderingOrdered, State: ibckeeper.StateInit}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateChannelState(ctx, storage, "transfer", "channel-0", ibckeeper.StateInit, ibckeeper.StateOpen); err != nil {
		t.Fatal(err)
	}
	if err := AcknowledgePacketWithProof(ctx, storage, 6, "07-vexo-0", packet, proof, []byte("ack")); err == nil {
		t.Fatalf("expected incomplete ack proof wrapper to fail")
	}
	if err := TimeoutPacketWithProof(ctx, storage, 10, "07-vexo-0", timeoutPacket, proof); err == nil {
		t.Fatalf("expected incomplete timeout proof wrapper to fail")
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
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
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

func TestModuleLifecycleTracksHeightAndSweepsExpiredPackets(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	hash := strings.Repeat("01", 32)
	setupCtx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:client-create:07-vexo-0:counterparty:5:" + hash),
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
		types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10"),
	} {
		if result := module.DeliverTx(setupCtx, tx); result.Code != 0 {
			t.Fatalf("setup %q failed: %+v", tx, result)
		}
	}
	blockCtx := vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}
	if err := module.BeginBlock(blockCtx, types.Header{Height: 10}); err != nil {
		t.Fatal(err)
	}
	encodedHeight, err := storage.Get(context.Background(), ibckeeper.Namespace, latestBeginHeightKey)
	if err != nil || string(encodedHeight) != "10" {
		t.Fatalf("expected begin height marker, got %q err=%v", encodedHeight, err)
	}
	if err := module.EndBlock(blockCtx); err != nil {
		t.Fatal(err)
	}
	packet := ibckeeper.Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload"), TimeoutHeight: 10}
	receipt, found, err := ibckeeper.NewKeeper(storage).PacketReceipt(context.Background(), packet)
	if err != nil || !found || !receipt.TimedOut || receipt.TimeoutAt != 10 {
		t.Fatalf("expected lifecycle timeout found=%t receipt=%+v err=%v", found, receipt, err)
	}
}

func TestModuleAcknowledgesPacketWithProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	remoteStore, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer remoteStore.Close()
	module := NewModule()
	hash := strings.Repeat("01", 32)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:client-create:07-vexo-0:counterparty:5:" + hash),
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
		types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA"),
	} {
		if result := module.DeliverTx(ctx, tx); result.Code != 0 {
			t.Fatalf("deliver %q failed: %+v", tx, result)
		}
	}
	packet := ibckeeper.Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload")}
	remoteReceipt := ibckeeper.PacketReceipt{Packet: packet, CommitHeight: 8, Acknowledged: true, Ack: []byte("ack"), AckHeight: 8}
	encoded, err := json.Marshal(remoteReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := remoteStore.Set(context.Background(), ibckeeper.Namespace, ibckeeper.PacketCommitmentKey(packet), encoded); err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.Build(context.Background(), remoteStore, "counterparty", 8, ibckeeper.Namespace, ibckeeper.PacketCommitmentKey(packet))
	if err != nil {
		t.Fatal(err)
	}
	update := types.Tx("ibc:client-update:07-vexo-0:8:" + hash + ":" + hex.EncodeToString(proof.StateRoot[:]))
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 8, Store: storage}, update); result.Code != 0 {
		t.Fatalf("client update failed: %+v", result)
	}
	proofArg := encodeProofForTest(t, proof)
	badProofArg := encodeProofForTest(t, queryproof.Proof{ChainID: proof.ChainID, Height: proof.Height, Namespace: proof.Namespace, Key: proof.Key, Exists: true, Value: []byte("tampered"), StateRoot: proof.StateRoot})
	badTx := types.Tx("ibc:packet-ack-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr:07-vexo-0:" + badProofArg)
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}, badTx); result.Code == 0 {
		t.Fatalf("expected tampered proof rejection")
	}
	tx := types.Tx("ibc:packet-ack-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr:07-vexo-0:" + proofArg)
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}, tx); result.Code != 0 {
		t.Fatalf("ack proof failed: %+v", result)
	}
	receipt, found, err := ibckeeper.NewKeeper(storage).PacketReceipt(context.Background(), packet)
	if err != nil || !found || !receipt.Acknowledged || string(receipt.Ack) != "ack" {
		t.Fatalf("unexpected receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
}

func TestModuleTimesOutPacketWithProof(t *testing.T) {
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
		types.Tx("ibc:connection-open-init:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:connection-open-ack:connection-0"),
		types.Tx("ibc:channel-open-init:transfer:channel-0:connection-0:channel-1:ordered"),
		types.Tx("ibc:channel-open-ack:transfer:channel-0"),
		types.Tx("ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10"),
	} {
		if result := module.DeliverTx(ctx, tx); result.Code != 0 {
			t.Fatalf("deliver %q failed: %+v", tx, result)
		}
	}
	packet := ibckeeper.Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestinationPort: "transfer", DestinationChannel: "channel-1", Data: []byte("payload"), TimeoutHeight: 10}
	proof, err := queryproof.Build(context.Background(), storage, "counterparty", 8, ibckeeper.Namespace, ibckeeper.PacketCommitmentKey(packet))
	if err != nil {
		t.Fatal(err)
	}
	update := types.Tx("ibc:client-update:07-vexo-0:8:" + hash + ":" + hex.EncodeToString(proof.StateRoot[:]))
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 8, Store: storage}, update); result.Code != 0 {
		t.Fatalf("client update failed: %+v", result)
	}
	tx := types.Tx("ibc:packet-timeout-proof:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10:07-vexo-0:" + encodeProofForTest(t, proof))
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}, tx); result.Code == 0 {
		t.Fatalf("expected early timeout rejection")
	}
	if result := module.DeliverTx(vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}, tx); result.Code != 0 {
		t.Fatalf("timeout proof failed: %+v", result)
	}
	receipt, found, err := ibckeeper.NewKeeper(storage).PacketReceipt(context.Background(), packet)
	if err != nil || !found || !receipt.TimedOut {
		t.Fatalf("unexpected timeout receipt found=%t receipt=%+v err=%v", found, receipt, err)
	}
}

func encodeProofForTest(t *testing.T, proof queryproof.Proof) string {
	t.Helper()
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawStdEncoding.EncodeToString(encoded)
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

func TestModuleLegacyOpenAliasesOnlyInitializeHandshake(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}
	for _, tx := range []types.Tx{
		types.Tx("ibc:connection-open:connection-0:07-vexo-0:connection-1"),
		types.Tx("ibc:channel-open:transfer:channel-0:connection-0:channel-1:ordered"),
	} {
		if result := module.DeliverTx(ctx, tx); result.Code != 0 {
			t.Fatalf("deliver %q failed: %+v", tx, result)
		}
	}
	keeper := ibckeeper.NewKeeper(storage)
	connection, found, err := keeper.Connection(context.Background(), "connection-0")
	if err != nil || !found || connection.State != ibckeeper.StateInit {
		t.Fatalf("expected legacy connection-open to initialize only, found=%t connection=%+v err=%v", found, connection, err)
	}
	channel, found, err := keeper.Channel(context.Background(), "transfer", "channel-0")
	if err != nil || !found || channel.State != ibckeeper.StateInit {
		t.Fatalf("expected legacy channel-open to initialize only, found=%t channel=%+v err=%v", found, channel, err)
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
	if commands := NewModule().CLICommands(); len(commands) != 1 || commands[0].Name != ModuleName {
		t.Fatalf("unexpected module commands: %+v", commands)
	}
	var output strings.Builder
	if err := command.Execute(&output, []string{"tx", "client-create", "07-vexo-0", "counterparty", "5", strings.Repeat("01", 32)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:client-create:07-vexo-0:counterparty:5:"+strings.Repeat("01", 32)) {
		t.Fatalf("unexpected client create output: %s", output.String())
	}
	output.Reset()
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
	for _, args := range [][]string{
		{"tx", "connection-open", "connection-0", "07-vexo-0", "connection-1"},
		{"tx", "connection-open-try", "connection-2", "07-vexo-0", "connection-3"},
		{"tx", "connection-open-ack", "connection-2"},
		{"tx", "connection-open-confirm", "connection-2"},
		{"tx", "channel-open", "transfer", "channel-0", "connection-0", "channel-1", "ordered"},
		{"tx", "channel-open-init", "transfer", "channel-0", "connection-0", "channel-1", "ordered"},
		{"tx", "channel-open-try", "transfer", "channel-2", "connection-2", "channel-3", "unordered"},
		{"tx", "channel-open-confirm", "transfer", "channel-2"},
		{"query", "client", "07-vexo-0"},
		{"query", "connection", "connection-0"},
		{"query", "channel", "transfer", "channel-0"},
		{"packet", "send", "--sequence", "9", "--source-port", "transfer", "--source-channel", "channel-0", "--destination-port", "transfer", "--destination-channel", "channel-1", "--data", "payload"},
	} {
		output.Reset()
		if err := command.Execute(&output, args); err != nil {
			t.Fatalf("command %v failed: %v", args, err)
		}
		if strings.TrimSpace(output.String()) == "" {
			t.Fatalf("expected output for command %v", args)
		}
	}
	proofArg := encodeProofForTest(t, queryproof.Proof{ChainID: "counterparty", Height: 1})
	for _, args := range [][]string{
		{"tx", "packet-ack-proof", "1", "transfer", "channel-0", "transfer", "channel-1", "payload", "ack", "07-vexo-0", proofArg},
		{"tx", "packet-timeout-proof", "1", "transfer", "channel-0", "transfer", "channel-1", "payload", "10", "07-vexo-0", proofArg},
	} {
		output.Reset()
		if err := command.Execute(&output, args); err != nil {
			t.Fatalf("proof command %v failed: %v", args, err)
		}
		if !strings.Contains(output.String(), proofArg) {
			t.Fatalf("expected proof arg in output for %v, got %s", args, output.String())
		}
	}
}
