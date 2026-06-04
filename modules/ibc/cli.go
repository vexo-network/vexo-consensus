package ibc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
)

func (Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{ibcCLICommand()}
}

func ibcCLICommand() vexoapp.CLICommand {
	return vexoapp.CLICommand{
		Name:        ModuleName,
		Usage:       "ibc <command>",
		Description: "IBC module commands for clients, connections, channels, and packets",
		Examples: []string{
			"ibc tx client-create 07-vexo-0 counterparty 10 <validator-set-hash>",
			"ibc tx client-update 07-vexo-0 11 <validator-set-hash> <state-root>",
			"ibc tx packet-send 1 transfer channel-0 transfer channel-1 payload",
			"ibc tx packet-ack 1 transfer channel-0 transfer channel-1 payload ack",
			"ibc tx packet-timeout 1 transfer channel-0 transfer channel-1 payload 100",
			"ibc query packet 1 transfer channel-0 transfer channel-1",
		},
		Children: []vexoapp.CLICommand{
			{
				Name:        "tx",
				Usage:       "ibc tx <command>",
				Description: "build IBC transaction payloads",
				Children: []vexoapp.CLICommand{
					{
						Name:        "client-create",
						Usage:       "ibc tx client-create <client_id> <chain_id> <latest_height> <validator_set_hash_hex> [state_root_hex]",
						Description: "build an IBC client creation transaction",
						Args: []vexoapp.CLIArg{
							{Name: "client_id", Description: "local client identifier"},
							{Name: "chain_id", Description: "counterparty chain id"},
							{Name: "latest_height", Description: "counterparty latest trusted height"},
							{Name: "validator_set_hash_hex", Description: "32-byte validator set hash"},
							{Name: "state_root_hex", Description: "optional trusted counterparty state root"},
						},
						Run: runClientCreateCLI,
					},
					{
						Name:        "client-update",
						Usage:       "ibc tx client-update <client_id> <latest_height> <validator_set_hash_hex> <state_root_hex>",
						Description: "build an IBC client update transaction",
						Args: []vexoapp.CLIArg{
							{Name: "client_id", Description: "local client identifier"},
							{Name: "latest_height", Description: "counterparty latest trusted height"},
							{Name: "validator_set_hash_hex", Description: "32-byte validator set hash"},
							{Name: "state_root_hex", Description: "32-byte counterparty state root"},
						},
						Run: runClientUpdateCLI,
					},
					{
						Name:        "connection-open",
						Usage:       "ibc tx connection-open <connection_id> <client_id> <counterparty>",
						Description: "build an IBC connection open transaction",
						Args: []vexoapp.CLIArg{
							{Name: "connection_id", Description: "local connection id"},
							{Name: "client_id", Description: "client id bound to this connection"},
							{Name: "counterparty", Description: "counterparty connection id"},
						},
						Run: runConnectionOpenCLI,
					},
					{
						Name:        "channel-open",
						Usage:       "ibc tx channel-open <port_id> <channel_id> <connection_id> <counterparty> <ordering>",
						Description: "build an IBC channel open transaction",
						Args: []vexoapp.CLIArg{
							{Name: "port_id", Description: "local port id"},
							{Name: "channel_id", Description: "local channel id"},
							{Name: "connection_id", Description: "connection id"},
							{Name: "counterparty", Description: "counterparty channel id"},
							{Name: "ordering", Description: "ordered or unordered"},
						},
						Run: runChannelOpenCLI,
					},
					{
						Name:        "packet-send",
						Usage:       "ibc tx packet-send <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> [timeout_height]",
						Description: "build an IBC packet send transaction",
						Args: []vexoapp.CLIArg{
							{Name: "sequence", Description: "packet sequence"},
							{Name: "source_port", Description: "source port"},
							{Name: "source_channel", Description: "source channel"},
							{Name: "destination_port", Description: "destination port"},
							{Name: "destination_channel", Description: "destination channel"},
							{Name: "data", Description: "packet data as plain text"},
							{Name: "timeout_height", Description: "optional timeout height"},
						},
						Run: runPacketSendTxCLI,
					},
					{
						Name:        "packet-ack",
						Usage:       "ibc tx packet-ack <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> [timeout_height] <ack>",
						Description: "build an IBC packet acknowledgement transaction",
						Args: []vexoapp.CLIArg{
							{Name: "sequence", Description: "packet sequence"},
							{Name: "source_port", Description: "source port"},
							{Name: "source_channel", Description: "source channel"},
							{Name: "destination_port", Description: "destination port"},
							{Name: "destination_channel", Description: "destination channel"},
							{Name: "data", Description: "original packet data as plain text"},
							{Name: "timeout_height", Description: "optional original timeout height"},
							{Name: "ack", Description: "acknowledgement bytes as plain text"},
						},
						Run: runPacketAckTxCLI,
					},
					{
						Name:        "packet-timeout",
						Usage:       "ibc tx packet-timeout <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> <timeout_height>",
						Description: "build an IBC packet timeout transaction",
						Args: []vexoapp.CLIArg{
							{Name: "sequence", Description: "packet sequence"},
							{Name: "source_port", Description: "source port"},
							{Name: "source_channel", Description: "source channel"},
							{Name: "destination_port", Description: "destination port"},
							{Name: "destination_channel", Description: "destination channel"},
							{Name: "data", Description: "original packet data as plain text"},
							{Name: "timeout_height", Description: "original timeout height"},
						},
						Run: runPacketTimeoutTxCLI,
					},
				},
			},
			{
				Name:        "query",
				Usage:       "ibc query <command>",
				Description: "build IBC query paths",
				Children: []vexoapp.CLICommand{
					{Name: "client", Usage: "ibc query client <client_id>", Description: "build an IBC client query path", Run: runClientQueryCLI},
					{Name: "connection", Usage: "ibc query connection <connection_id>", Description: "build an IBC connection query path", Run: runConnectionQueryCLI},
					{Name: "channel", Usage: "ibc query channel <port_id> <channel_id>", Description: "build an IBC channel query path", Run: runChannelQueryCLI},
					{Name: "packet", Usage: "ibc query packet <sequence> <source_port> <source_channel> <destination_port> <destination_channel>", Description: "build an IBC packet receipt query path", Run: runPacketQueryCLI},
				},
			},
			{
				Name:        "packet",
				Usage:       "ibc packet <command>",
				Description: "build raw IBC packet scaffolds",
				Children: []vexoapp.CLICommand{
					{Name: "send", Usage: "ibc packet send --sequence <n> --source-port <port> --source-channel <channel> --destination-port <port> --destination-channel <channel> --data <payload>", Description: "build a JSON or text IBC packet scaffold", Run: runPacketScaffoldCLI},
				},
			},
		},
	}
}

func runClientCreateCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 4 && len(args) != 5 {
		return vexoapp.ErrCLIUsage("ibc tx client-create <client_id> <chain_id> <latest_height> <validator_set_hash_hex> [state_root_hex]")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "client-create", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runClientUpdateCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 4 {
		return vexoapp.ErrCLIUsage("ibc tx client-update <client_id> <latest_height> <validator_set_hash_hex> <state_root_hex>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "client-update", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runConnectionOpenCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 3 {
		return vexoapp.ErrCLIUsage("ibc tx connection-open <connection_id> <client_id> <counterparty>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "connection-open", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runChannelOpenCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 5 {
		return vexoapp.ErrCLIUsage("ibc tx channel-open <port_id> <channel_id> <connection_id> <counterparty> <ordering>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "channel-open", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runPacketSendTxCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 6 && len(args) != 7 {
		return vexoapp.ErrCLIUsage("ibc tx packet-send <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> [timeout_height]")
	}
	txArgs := append([]string(nil), args...)
	txArgs[5] = base64.RawStdEncoding.EncodeToString([]byte(args[5]))
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "packet-send", Args: txArgs, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runPacketAckTxCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 7 && len(args) != 8 {
		return vexoapp.ErrCLIUsage("ibc tx packet-ack <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> [timeout_height] <ack>")
	}
	txArgs := append([]string(nil), args...)
	dataIndex := 5
	ackIndex := len(txArgs) - 1
	txArgs[dataIndex] = base64.RawStdEncoding.EncodeToString([]byte(args[dataIndex]))
	txArgs[ackIndex] = base64.RawStdEncoding.EncodeToString([]byte(args[ackIndex]))
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "packet-ack", Args: txArgs, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runPacketTimeoutTxCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 7 {
		return vexoapp.ErrCLIUsage("ibc tx packet-timeout <sequence> <source_port> <source_channel> <destination_port> <destination_channel> <data> <timeout_height>")
	}
	txArgs := append([]string(nil), args...)
	txArgs[5] = base64.RawStdEncoding.EncodeToString([]byte(args[5]))
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "packet-timeout", Args: txArgs, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runClientQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("ibc query client <client_id>")
	}
	fmt.Fprintf(writer, "query_path: %s/client/%s\n", ModuleName, args[0])
	return nil
}

func runConnectionQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("ibc query connection <connection_id>")
	}
	fmt.Fprintf(writer, "query_path: %s/connection/%s\n", ModuleName, args[0])
	return nil
}

func runChannelQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("ibc query channel <port_id> <channel_id>")
	}
	fmt.Fprintf(writer, "query_path: %s/channel/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func runPacketQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 5 {
		return vexoapp.ErrCLIUsage("ibc query packet <sequence> <source_port> <source_channel> <destination_port> <destination_channel>")
	}
	if _, err := strconv.ParseUint(args[0], 10, 64); err != nil {
		return ErrInvalidIBCTx
	}
	fmt.Fprintf(writer, "query_path: %s/packet/%s/%s/%s/%s/%s\n", ModuleName, args[0], args[1], args[2], args[3], args[4])
	return nil
}

func runPacketScaffoldCLI(writer io.Writer, args []string) error {
	packet, jsonOutput, err := parsePacketScaffoldArgs(args)
	if err != nil {
		return err
	}
	if jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(packet)
	}
	fmt.Fprintf(writer, "ibc_packet: %s\n", base64.StdEncoding.EncodeToString(packet.Data))
	fmt.Fprintf(writer, "sequence: %d\n", packet.Sequence)
	fmt.Fprintf(writer, "source: %s/%s\n", packet.SourcePort, packet.SourceChannel)
	fmt.Fprintf(writer, "destination: %s/%s\n", packet.DestinationPort, packet.DestinationChannel)
	if packet.TimeoutHeight > 0 {
		fmt.Fprintf(writer, "timeout_height: %d\n", packet.TimeoutHeight)
	}
	return nil
}

func parsePacketScaffoldArgs(args []string) (ibckeeper.Packet, bool, error) {
	values := map[string]string{}
	jsonOutput := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			jsonOutput = true
		case "--sequence", "--source-port", "--source-channel", "--destination-port", "--destination-channel", "--data", "--timeout-height":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return ibckeeper.Packet{}, false, vexoapp.ErrCLIUsage(arg + " <value>")
			}
			values[arg] = args[index+1]
			index++
		default:
			return ibckeeper.Packet{}, false, vexoapp.ErrCLIUsage("unknown ibc packet flag " + arg)
		}
	}
	sequence, err := strconv.ParseUint(values["--sequence"], 10, 64)
	if err != nil || sequence == 0 {
		return ibckeeper.Packet{}, false, ErrInvalidIBCTx
	}
	timeoutHeight := uint64(0)
	if values["--timeout-height"] != "" {
		timeoutHeight, err = strconv.ParseUint(values["--timeout-height"], 10, 64)
		if err != nil {
			return ibckeeper.Packet{}, false, ErrInvalidIBCTx
		}
	}
	packet := ibckeeper.Packet{
		Sequence:           sequence,
		SourcePort:         values["--source-port"],
		SourceChannel:      values["--source-channel"],
		DestinationPort:    values["--destination-port"],
		DestinationChannel: values["--destination-channel"],
		Data:               []byte(values["--data"]),
		TimeoutHeight:      timeoutHeight,
	}
	if err := ibckeeper.ValidatePacket(packet); err != nil {
		return ibckeeper.Packet{}, false, err
	}
	return packet, jsonOutput, nil
}

func splitExecutionTags(args []string) ([]string, map[string]string, error) {
	positional := make([]string, 0, len(args))
	tags := make(map[string]string)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		switch key {
		case "fee", "gas", "signer", "nonce":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return nil, nil, vexoapp.ErrCLIUsage("--" + key + " <value>")
			}
			tags[key] = args[index+1]
			index++
		default:
			return nil, nil, vexoapp.ErrCLIUsage("unknown ibc tx flag " + arg)
		}
	}
	if len(tags) == 0 {
		return positional, nil, nil
	}
	return positional, tags, nil
}
