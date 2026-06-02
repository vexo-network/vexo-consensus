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
	fmt.Fprintf(writer, "p2p.initial_score: %d\n", cfg.P2P.InitialScore)
	fmt.Fprintf(writer, "p2p.valid_message_reward: %d\n", cfg.P2P.ValidMessageReward)
	fmt.Fprintf(writer, "p2p.invalid_message_cost: %d\n", cfg.P2P.InvalidMessageCost)
	fmt.Fprintf(writer, "p2p.rate_limit_cost: %d\n", cfg.P2P.RateLimitCost)
	fmt.Fprintf(writer, "p2p.ban_threshold: %d\n", cfg.P2P.BanThreshold)
	fmt.Fprintf(writer, "p2p.max_messages_per_window: %d\n", cfg.P2P.MaxMessagesPerWindow)
	fmt.Fprintf(writer, "p2p.window_reset_interval: %s\n", cfg.P2P.WindowResetInterval)
	fmt.Fprintf(writer, "p2p.score_recovery: %d\n", cfg.P2P.ScoreRecovery)
	fmt.Fprintf(writer, "p2p.ban_duration: %s\n", cfg.P2P.BanDuration)
}
