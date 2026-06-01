package main

import (
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/config"
)

func writeStatus(writer io.Writer, cfg config.Config) {
	fmt.Fprintf(writer, "vexo-consensus status\n")
	fmt.Fprintf(writer, "chain_id: %s\n", cfg.ChainID)
	fmt.Fprintf(writer, "validator.permissionless: %t\n", cfg.Validator.Permissionless)
	fmt.Fprintf(writer, "validator.min_stake: %d\n", cfg.Validator.MinStake)
	fmt.Fprintf(writer, "committee.epoch_length: %d\n", cfg.Committee.EpochLength)
	fmt.Fprintf(writer, "committee.size: %d\n", cfg.Committee.CommitteeSize)
	fmt.Fprintf(writer, "mempool.max_txs: %d\n", cfg.Mempool.MaxTxs)
	fmt.Fprintf(writer, "fair_ordering.deterministic: true\n")
	fmt.Fprintf(writer, "data_availability.commitments: true\n")
	fmt.Fprintf(writer, "storage.backend: leveldb\n")
	fmt.Fprintf(writer, "p2p.max_messages_per_window: %d\n", cfg.P2P.MaxMessagesPerWindow)
}
