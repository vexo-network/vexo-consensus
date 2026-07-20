package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type RecoveryReport struct {
	OK                bool
	Running           bool
	LatestHeight      types.Height
	LatestStateHeight types.Height
	SafeHeight        types.Height
	EarliestBlock     types.Height
	LatestBlock       types.Height
	TotalBlocks       uint64
	SnapshotAvailable bool
	Repaired          bool
	RecoverResult     store.RecoverResult
	Problems          []string
}

func (node *Node) RecoveryReport(ctx context.Context, repairIndexes bool) (RecoveryReport, error) {
	status := node.Status(ctx)
	report := RecoveryReport{
		OK:           true,
		Running:      status.Running,
		LatestHeight: status.LatestHeight,
	}
	runtime, err := node.Runtime()
	if err != nil {
		report.OK = false
		report.Problems = append(report.Problems, err.Error())
		return report, err
	}
	if repairIndexes {
		result, err := runtime.RecoverIndexes(ctx)
		if err != nil {
			report.addProblem(err)
		} else {
			report.Repaired = true
			report.RecoverResult = result
		}
	}
	state, err := runtime.LatestState(ctx)
	if err != nil {
		report.addProblem(err)
	} else {
		report.LatestStateHeight = state.Height
	}
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		if !errors.Is(err, store.ErrBlockIndexNotFound) {
			report.addProblem(err)
		} else {
			report.addProblem(err)
		}
	} else {
		report.EarliestBlock = index.EarliestHeight
		report.LatestBlock = index.LatestHeight
		report.TotalBlocks = index.TotalBlocks
	}
	if _, err := node.StateSnapshot(ctx); err != nil {
		report.addProblem(err)
	} else {
		report.SnapshotAvailable = true
	}
	report.SafeHeight = safeRecoveryHeight(report.LatestStateHeight, report.LatestBlock)
	if report.LatestStateHeight > 0 && report.LatestBlock > 0 && report.LatestStateHeight != report.LatestBlock {
		report.addProblem(fmt.Errorf("recovery height mismatch: state=%d block=%d safe=%d", report.LatestStateHeight, report.LatestBlock, report.SafeHeight))
	}
	return report, nil
}

func (report *RecoveryReport) addProblem(err error) {
	report.OK = false
	report.Problems = append(report.Problems, err.Error())
}

func safeRecoveryHeight(stateHeight types.Height, blockHeight types.Height) types.Height {
	if stateHeight == 0 {
		return 0
	}
	if blockHeight == 0 || blockHeight > stateHeight {
		return stateHeight
	}
	return blockHeight
}

func recoveryFinalityAllowsCommit(report RecoveryReport, committedHeight types.Height) bool {
	if report.LatestStateHeight == 0 || report.LatestBlock == 0 {
		return true
	}
	if report.LatestStateHeight == report.LatestBlock {
		return true
	}
	return committedHeight <= report.SafeHeight
}
