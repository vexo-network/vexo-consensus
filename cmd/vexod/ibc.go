package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/ibc"
)

func runIBC(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("ibc subcommand is required")
	}
	switch args[0] {
	case "packet":
		return runIBCPacket(writer, args[1:])
	default:
		return fmt.Errorf("unknown ibc subcommand %q", args[0])
	}
}

func runIBCPacket(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("ibc packet subcommand is required")
	}
	switch args[0] {
	case "send":
		return runIBCPacketSend(writer, args[1:])
	default:
		return fmt.Errorf("unknown ibc packet subcommand %q", args[0])
	}
}

func runIBCPacketSend(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("ibc packet send", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sequence := flags.Uint64("sequence", 0, "packet sequence")
	sourcePort := flags.String("source-port", "", "source port")
	sourceChannel := flags.String("source-channel", "", "source channel")
	destinationPort := flags.String("destination-port", "", "destination port")
	destinationChannel := flags.String("destination-channel", "", "destination channel")
	data := flags.String("data", "", "packet data")
	timeoutHeight := flags.Uint64("timeout-height", 0, "timeout height")
	jsonOutput := flags.Bool("json", false, "write JSON packet")
	if err := flags.Parse(args); err != nil {
		return err
	}
	packet := ibc.Packet{
		Sequence:           *sequence,
		SourcePort:         *sourcePort,
		SourceChannel:      *sourceChannel,
		DestinationPort:    *destinationPort,
		DestinationChannel: *destinationChannel,
		Data:               []byte(*data),
		TimeoutHeight:      *timeoutHeight,
	}
	if err := ibc.ValidatePacket(packet); err != nil {
		return err
	}
	if *jsonOutput {
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
