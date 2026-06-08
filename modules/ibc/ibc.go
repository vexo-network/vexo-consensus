package ibc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "ibc"

const (
	clientCreateGasCost   uint64 = 20
	clientUpdateGasCost   uint64 = 20
	connectionOpenGasCost uint64 = 15
	connectionAckGasCost  uint64 = 10
	channelOpenGasCost    uint64 = 15
	channelAckGasCost     uint64 = 10
	packetSendGasCost     uint64 = 25
	packetAckGasCost      uint64 = 20
	packetTimeoutGasCost  uint64 = 20
)

var latestBeginHeightKey = []byte("meta/latest_begin_height")

var (
	ErrInvalidIBCTx    = errors.New("invalid IBC transaction")
	ErrInvalidIBCQuery = errors.New("invalid IBC query")
	ErrStoreMissing    = errors.New("IBC module store is required")
)

type Module struct{}

func NewModule() Module { return Module{} }

func (Module) Name() string { return ModuleName }

func (Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	keeper := ibckeeper.NewKeeper(ctx.Store)
	for rawKey, rawValue := range genesis {
		switch {
		case strings.HasPrefix(rawKey, ModuleName+":client:"):
			var client ibckeeper.ClientState
			if err := json.Unmarshal(rawValue, &client); err != nil {
				return err
			}
			if err := keeper.SetClient(ctx.GoContext(), client); err != nil {
				return err
			}
		case strings.HasPrefix(rawKey, ModuleName+":connection:"):
			var connection ibckeeper.ConnectionState
			if err := json.Unmarshal(rawValue, &connection); err != nil {
				return err
			}
			if err := keeper.SetConnection(ctx.GoContext(), connection); err != nil {
				return err
			}
		case strings.HasPrefix(rawKey, ModuleName+":channel:"):
			var channel ibckeeper.ChannelState
			if err := json.Unmarshal(rawValue, &channel); err != nil {
				return err
			}
			if err := keeper.SetChannel(ctx.GoContext(), channel); err != nil {
				return err
			}
		}
	}
	return nil
}

func (Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	if ctx.Store == nil {
		return nil
	}
	height := header.Height
	if height == 0 {
		height = ctx.Height
	}
	if height == 0 {
		return nil
	}
	return ctx.Store.Set(ctx.GoContext(), ibckeeper.Namespace, latestBeginHeightKey, []byte(strconv.FormatUint(uint64(height), 10)))
}

func (Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: ErrStoreMissing.Error()}
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
	keeper := ibckeeper.NewKeeper(ctx.Store)
	switch canonical.Action {
	case "client-create":
		if err := ctx.ConsumeGas(clientCreateGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		if len(canonical.Args) != 4 && len(canonical.Args) != 5 {
			return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
		}
		height, err := parseHeight(canonical.Args[2])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		hash, err := parseHash(canonical.Args[3])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		stateRoot := types.Hash{}
		if len(canonical.Args) == 5 {
			stateRoot, err = parseHash(canonical.Args[4])
			if err != nil {
				return types.Result{Code: 3, Log: err.Error()}
			}
		}
		err = keeper.SetClient(ctx.GoContext(), ibckeeper.ClientState{
			ClientID:         canonical.Args[0],
			ChainID:          canonical.Args[1],
			LatestHeight:     height,
			ValidatorSetHash: hash,
			LatestStateRoot:  stateRoot,
		})
		return resultFromError(err)
	case "client-update":
		if err := ctx.ConsumeGas(clientUpdateGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		if len(canonical.Args) != 4 {
			return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
		}
		height, err := parseHeight(canonical.Args[1])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		validatorSetHash, err := parseHash(canonical.Args[2])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		stateRoot, err := parseHash(canonical.Args[3])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.UpdateClient(ctx.GoContext(), canonical.Args[0], height, validatorSetHash, stateRoot)
		return resultFromError(err)
	case "connection-open":
		return deliverConnectionOpen(ctx, keeper, canonical.Args, ibckeeper.StateInit)
	case "connection-open-init":
		return deliverConnectionOpen(ctx, keeper, canonical.Args, ibckeeper.StateInit)
	case "connection-open-try":
		return deliverConnectionOpen(ctx, keeper, canonical.Args, ibckeeper.StateTryOpen)
	case "connection-open-ack":
		return deliverConnectionTransition(ctx, keeper, canonical.Args, ibckeeper.StateInit, ibckeeper.StateOpen)
	case "connection-open-confirm":
		return deliverConnectionTransition(ctx, keeper, canonical.Args, ibckeeper.StateTryOpen, ibckeeper.StateOpen)
	case "channel-open":
		return deliverChannelOpen(ctx, keeper, canonical.Args, ibckeeper.StateInit)
	case "channel-open-init":
		return deliverChannelOpen(ctx, keeper, canonical.Args, ibckeeper.StateInit)
	case "channel-open-try":
		return deliverChannelOpen(ctx, keeper, canonical.Args, ibckeeper.StateTryOpen)
	case "channel-open-ack":
		return deliverChannelTransition(ctx, keeper, canonical.Args, ibckeeper.StateInit, ibckeeper.StateOpen)
	case "channel-open-confirm":
		return deliverChannelTransition(ctx, keeper, canonical.Args, ibckeeper.StateTryOpen, ibckeeper.StateOpen)
	case "packet-send":
		if err := ctx.ConsumeGas(packetSendGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		packet, err := packetFromArgs(canonical.Args)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.SendPacket(ctx.GoContext(), ctx.Height, packet)
		return resultFromError(err)
	case "packet-ack":
		if err := ctx.ConsumeGas(packetAckGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		packet, ack, err := packetAckFromArgs(canonical.Args)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.AcknowledgePacket(ctx.GoContext(), ctx.Height, packet, ack)
		return resultFromError(err)
	case "packet-ack-proof":
		if err := ctx.ConsumeGas(packetAckGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		packet, ack, clientID, proof, err := packetAckProofFromArgs(canonical.Args)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.AcknowledgePacketWithProof(ctx.GoContext(), ctx.Height, clientID, packet, proof, ack)
		return resultFromError(err)
	case "packet-timeout":
		if err := ctx.ConsumeGas(packetTimeoutGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		packet, err := packetFromArgs(canonical.Args)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.TimeoutPacket(ctx.GoContext(), ctx.Height, packet)
		return resultFromError(err)
	case "packet-timeout-proof":
		if err := ctx.ConsumeGas(packetTimeoutGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		packet, clientID, proof, err := packetTimeoutProofFromArgs(canonical.Args)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		err = keeper.TimeoutPacketWithProof(ctx.GoContext(), ctx.Height, clientID, packet, proof)
		return resultFromError(err)
	default:
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
}

func (Module) EndBlock(ctx vexoapp.Context) error {
	if ctx.Store == nil || ctx.Height == 0 {
		return nil
	}
	return sweepExpiredPackets(ctx)
}

func sweepExpiredPackets(ctx vexoapp.Context) error {
	snapshot, ok := ctx.Store.(store.SnapshotKVStore)
	if !ok {
		return nil
	}
	pairs, err := snapshot.ExportNamespace(ctx.GoContext(), ibckeeper.Namespace)
	if err != nil {
		return err
	}
	keeper := ibckeeper.NewKeeper(ctx.Store)
	for _, pair := range pairs {
		if !strings.HasPrefix(string(pair.Key), "packets/") {
			continue
		}
		var receipt ibckeeper.PacketReceipt
		if err := json.Unmarshal(pair.Value, &receipt); err != nil {
			return err
		}
		if receipt.Acknowledged || receipt.TimedOut || receipt.Packet.TimeoutHeight == 0 || uint64(ctx.Height) < receipt.Packet.TimeoutHeight {
			continue
		}
		if err := keeper.TimeoutPacket(ctx.GoContext(), ctx.Height, receipt.Packet); err != nil && !errors.Is(err, ibckeeper.ErrPacketTimedOut) && !errors.Is(err, ibckeeper.ErrPacketAcked) {
			return err
		}
	}
	return nil
}

func (Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return 0, ErrInvalidIBCTx
	}
	switch canonical.Action {
	case "client-create":
		return clientCreateGasCost, nil
	case "client-update":
		return clientUpdateGasCost, nil
	case "connection-open":
		return connectionOpenGasCost, nil
	case "connection-open-init", "connection-open-try":
		return connectionOpenGasCost, nil
	case "connection-open-ack", "connection-open-confirm":
		return connectionAckGasCost, nil
	case "channel-open":
		return channelOpenGasCost, nil
	case "channel-open-init", "channel-open-try":
		return channelOpenGasCost, nil
	case "channel-open-ack", "channel-open-confirm":
		return channelAckGasCost, nil
	case "packet-send":
		return packetSendGasCost, nil
	case "packet-ack", "packet-ack-proof":
		return packetAckGasCost, nil
	case "packet-timeout", "packet-timeout-proof":
		return packetTimeoutGasCost, nil
	default:
		return 0, ErrInvalidIBCTx
	}
}

func (Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStoreMissing.Error()}
	}
	if len(req.Path) == 0 {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
	}
	keeper := ibckeeper.NewKeeper(ctx.Store)
	var value any
	var found bool
	var err error
	switch req.Path[0] {
	case "client":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
		}
		value, found, err = keeper.Client(ctx.GoContext(), req.Path[1])
	case "connection":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
		}
		value, found, err = keeper.Connection(ctx.GoContext(), req.Path[1])
	case "channel":
		if len(req.Path) != 3 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
		}
		value, found, err = keeper.Channel(ctx.GoContext(), req.Path[1], req.Path[2])
	case "packet":
		if len(req.Path) != 6 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
		}
		sequence, parseErr := strconv.ParseUint(req.Path[1], 10, 64)
		if parseErr != nil || sequence == 0 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
		}
		packet := ibckeeper.Packet{
			Sequence:           sequence,
			SourcePort:         req.Path[2],
			SourceChannel:      req.Path[3],
			DestinationPort:    req.Path[4],
			DestinationChannel: req.Path[5],
			Data:               []byte("query"),
		}
		value, found, err = keeper.PacketReceipt(ctx.GoContext(), packet)
	default:
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidIBCQuery.Error()}
	}
	if errors.Is(err, store.ErrKeyNotFound) || !found {
		return vexoapp.QueryResponse{Code: 3, Log: "IBC state not found"}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func (Module) Events(ctx vexoapp.Context, tx types.Tx, result types.Result) []events.Event {
	if result.Code != 0 {
		return nil
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return nil
	}
	attributes := []events.Attribute{{Key: "module", Value: ModuleName, Index: true}, {Key: "action", Value: canonical.Action, Index: true}}
	if len(canonical.Args) > 0 {
		attributes = append(attributes, events.Attribute{Key: "id", Value: canonical.Args[0], Index: true})
	}
	if packetAttributes, ok := packetEventAttributes(canonical.Action, canonical.Args); ok {
		attributes = append(attributes, packetAttributes...)
	}
	return []events.Event{{Type: "ibc_" + canonical.Action, Attributes: attributes}}
}

func packetEventAttributes(action string, args []string) ([]events.Attribute, bool) {
	var packet ibckeeper.Packet
	var ack []byte
	var err error
	packetEvent := ""
	switch action {
	case "packet-send":
		packet, err = packetFromArgs(args)
		packetEvent = "send"
	case "packet-ack":
		packet, ack, err = packetAckFromArgs(args)
		packetEvent = "ack"
	case "packet-ack-proof":
		packet, ack, _, _, err = packetAckProofFromArgs(args)
		packetEvent = "ack"
	case "packet-timeout":
		packet, err = packetFromArgs(args)
		packetEvent = "timeout"
	case "packet-timeout-proof":
		packet, _, _, err = packetTimeoutProofFromArgs(args)
		packetEvent = "timeout"
	default:
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	packetID := packet.SourcePort + "/" + packet.SourceChannel + "/" + strconv.FormatUint(packet.Sequence, 10)
	attributes := []events.Attribute{
		{Key: "ibc_packet_event", Value: packetEvent, Index: true},
		{Key: "ibc_packet_id", Value: packetID, Index: true},
		{Key: "ibc_sequence", Value: strconv.FormatUint(packet.Sequence, 10), Index: true},
		{Key: "ibc_source_port", Value: packet.SourcePort, Index: true},
		{Key: "ibc_source_channel", Value: packet.SourceChannel, Index: true},
		{Key: "ibc_destination_port", Value: packet.DestinationPort, Index: true},
		{Key: "ibc_destination_channel", Value: packet.DestinationChannel, Index: true},
		{Key: "ibc_data", Value: base64.RawStdEncoding.EncodeToString(packet.Data), Index: false},
	}
	if packet.TimeoutHeight > 0 {
		attributes = append(attributes, events.Attribute{Key: "ibc_timeout_height", Value: strconv.FormatUint(packet.TimeoutHeight, 10), Index: true})
	}
	if len(ack) > 0 {
		attributes = append(attributes, events.Attribute{Key: "ibc_ack", Value: base64.RawStdEncoding.EncodeToString(ack), Index: false})
	}
	return attributes, true
}

func resultFromError(err error) types.Result {
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	return types.Result{}
}

func deliverConnectionOpen(ctx vexoapp.Context, keeper *ibckeeper.Keeper, args []string, state string) types.Result {
	if err := ctx.ConsumeGas(connectionOpenGasCost); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if len(args) != 3 {
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
	err := keeper.SetConnection(ctx.GoContext(), ibckeeper.ConnectionState{
		ConnectionID: args[0],
		ClientID:     args[1],
		Counterparty: args[2],
		State:        state,
	})
	return resultFromError(err)
}

func deliverConnectionTransition(ctx vexoapp.Context, keeper *ibckeeper.Keeper, args []string, expectedState string, nextState string) types.Result {
	if err := ctx.ConsumeGas(connectionAckGasCost); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if len(args) != 1 {
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
	return resultFromError(keeper.UpdateConnectionState(ctx.GoContext(), args[0], expectedState, nextState))
}

func deliverChannelOpen(ctx vexoapp.Context, keeper *ibckeeper.Keeper, args []string, state string) types.Result {
	if err := ctx.ConsumeGas(channelOpenGasCost); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if len(args) != 5 {
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
	err := keeper.SetChannel(ctx.GoContext(), ibckeeper.ChannelState{
		PortID:       args[0],
		ChannelID:    args[1],
		ConnectionID: args[2],
		Counterparty: args[3],
		Ordering:     args[4],
		State:        state,
	})
	return resultFromError(err)
}

func deliverChannelTransition(ctx vexoapp.Context, keeper *ibckeeper.Keeper, args []string, expectedState string, nextState string) types.Result {
	if err := ctx.ConsumeGas(channelAckGasCost); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if len(args) != 2 {
		return types.Result{Code: 2, Log: ErrInvalidIBCTx.Error()}
	}
	return resultFromError(keeper.UpdateChannelState(ctx.GoContext(), args[0], args[1], expectedState, nextState))
}

func parseHeight(value string) (types.Height, error) {
	height, err := strconv.ParseUint(value, 10, 64)
	if err != nil || height == 0 {
		return 0, ErrInvalidIBCTx
	}
	return types.Height(height), nil
}

func parseHash(value string) (types.Hash, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(types.Hash{}) {
		return types.Hash{}, ErrInvalidIBCTx
	}
	var hash types.Hash
	copy(hash[:], decoded)
	return hash, nil
}

func packetFromArgs(args []string) (ibckeeper.Packet, error) {
	if len(args) != 6 && len(args) != 7 {
		return ibckeeper.Packet{}, ErrInvalidIBCTx
	}
	sequence, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil || sequence == 0 {
		return ibckeeper.Packet{}, ErrInvalidIBCTx
	}
	data, err := decodePacketData(args[5])
	if err != nil {
		return ibckeeper.Packet{}, err
	}
	timeoutHeight := uint64(0)
	if len(args) == 7 {
		timeoutHeight, err = strconv.ParseUint(args[6], 10, 64)
		if err != nil {
			return ibckeeper.Packet{}, ErrInvalidIBCTx
		}
	}
	packet := ibckeeper.Packet{
		Sequence:           sequence,
		SourcePort:         args[1],
		SourceChannel:      args[2],
		DestinationPort:    args[3],
		DestinationChannel: args[4],
		Data:               data,
		TimeoutHeight:      timeoutHeight,
	}
	if err := ibckeeper.ValidatePacket(packet); err != nil {
		return ibckeeper.Packet{}, err
	}
	return packet, nil
}

func packetAckFromArgs(args []string) (ibckeeper.Packet, []byte, error) {
	if len(args) != 7 && len(args) != 8 {
		return ibckeeper.Packet{}, nil, ErrInvalidIBCTx
	}
	packet, err := packetFromArgs(args[:len(args)-1])
	if err != nil {
		return ibckeeper.Packet{}, nil, err
	}
	ack, err := decodePacketData(args[len(args)-1])
	if err != nil || len(ack) == 0 {
		return ibckeeper.Packet{}, nil, ibckeeper.ErrInvalidAck
	}
	return packet, ack, nil
}

func packetAckProofFromArgs(args []string) (ibckeeper.Packet, []byte, string, queryproof.Proof, error) {
	if len(args) != 9 && len(args) != 10 {
		return ibckeeper.Packet{}, nil, "", queryproof.Proof{}, ErrInvalidIBCTx
	}
	packet, ack, err := packetAckFromArgs(args[:len(args)-2])
	if err != nil {
		return ibckeeper.Packet{}, nil, "", queryproof.Proof{}, err
	}
	clientID := args[len(args)-2]
	if clientID == "" {
		return ibckeeper.Packet{}, nil, "", queryproof.Proof{}, ibckeeper.ErrInvalidClient
	}
	proof, err := decodeProofData(args[len(args)-1])
	if err != nil {
		return ibckeeper.Packet{}, nil, "", queryproof.Proof{}, err
	}
	return packet, ack, clientID, proof, nil
}

func packetTimeoutProofFromArgs(args []string) (ibckeeper.Packet, string, queryproof.Proof, error) {
	if len(args) != 9 {
		return ibckeeper.Packet{}, "", queryproof.Proof{}, ErrInvalidIBCTx
	}
	packet, err := packetFromArgs(args[:len(args)-2])
	if err != nil {
		return ibckeeper.Packet{}, "", queryproof.Proof{}, err
	}
	clientID := args[len(args)-2]
	if clientID == "" {
		return ibckeeper.Packet{}, "", queryproof.Proof{}, ibckeeper.ErrInvalidClient
	}
	proof, err := decodeProofData(args[len(args)-1])
	if err != nil {
		return ibckeeper.Packet{}, "", queryproof.Proof{}, err
	}
	return packet, clientID, proof, nil
}

func decodePacketData(value string) ([]byte, error) {
	data, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil {
		return data, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func decodeProofData(value string) (queryproof.Proof, error) {
	data, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		data, err = base64.StdEncoding.DecodeString(value)
	}
	if err != nil {
		return queryproof.Proof{}, err
	}
	var proof queryproof.Proof
	if err := json.Unmarshal(data, &proof); err != nil {
		return queryproof.Proof{}, err
	}
	return proof, nil
}

func PacketQueryPath(packet ibckeeper.Packet) string {
	return ModuleName + "/packet/" + strconv.FormatUint(packet.Sequence, 10) + "/" + packet.SourcePort + "/" + packet.SourceChannel + "/" + packet.DestinationPort + "/" + packet.DestinationChannel
}

func NewKeeper(store vexoapp.StateStore) *ibckeeper.Keeper {
	return ibckeeper.NewKeeper(store)
}

func SendPacket(ctx context.Context, store vexoapp.StateStore, height types.Height, packet ibckeeper.Packet) error {
	return ibckeeper.NewKeeper(store).SendPacket(ctx, height, packet)
}

func UpdateClient(ctx context.Context, store vexoapp.StateStore, clientID string, latestHeight types.Height, validatorSetHash types.Hash, latestStateRoot types.Hash) error {
	return ibckeeper.NewKeeper(store).UpdateClient(ctx, clientID, latestHeight, validatorSetHash, latestStateRoot)
}

func VerifyClientProof(ctx context.Context, store vexoapp.StateStore, clientID string, proof queryproof.Proof) error {
	return ibckeeper.NewKeeper(store).VerifyClientProof(ctx, clientID, proof)
}

func UpdateConnectionState(ctx context.Context, store vexoapp.StateStore, connectionID string, expectedState string, nextState string) error {
	return ibckeeper.NewKeeper(store).UpdateConnectionState(ctx, connectionID, expectedState, nextState)
}

func UpdateChannelState(ctx context.Context, store vexoapp.StateStore, portID string, channelID string, expectedState string, nextState string) error {
	return ibckeeper.NewKeeper(store).UpdateChannelState(ctx, portID, channelID, expectedState, nextState)
}

func AcknowledgePacket(ctx context.Context, store vexoapp.StateStore, height types.Height, packet ibckeeper.Packet, ack []byte) error {
	return ibckeeper.NewKeeper(store).AcknowledgePacket(ctx, height, packet, ack)
}

func AcknowledgePacketWithProof(ctx context.Context, store vexoapp.StateStore, height types.Height, clientID string, packet ibckeeper.Packet, proof queryproof.Proof, ack []byte) error {
	return ibckeeper.NewKeeper(store).AcknowledgePacketWithProof(ctx, height, clientID, packet, proof, ack)
}

func TimeoutPacket(ctx context.Context, store vexoapp.StateStore, height types.Height, packet ibckeeper.Packet) error {
	return ibckeeper.NewKeeper(store).TimeoutPacket(ctx, height, packet)
}

func TimeoutPacketWithProof(ctx context.Context, store vexoapp.StateStore, height types.Height, clientID string, packet ibckeeper.Packet, proof queryproof.Proof) error {
	return ibckeeper.NewKeeper(store).TimeoutPacketWithProof(ctx, height, clientID, packet, proof)
}
