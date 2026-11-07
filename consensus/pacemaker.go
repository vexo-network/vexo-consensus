package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrStaleTimeoutCert = errors.New("stale timeout certificate")

// Pacemaker tracks the current height and the current global view. Neither ever moves
// backwards: views keep counting up across heights, so the round is never reset.
type Pacemaker struct {
	height types.Height
	round  types.Round
}

func NewPacemaker(height types.Height, round types.Round) *Pacemaker {
	return &Pacemaker{height: height, round: round}
}

func (pacemaker *Pacemaker) Height() types.Height {
	return pacemaker.height
}

func (pacemaker *Pacemaker) Round() types.Round {
	return pacemaker.round
}

// AdvanceRound leaves the timed-out view of timeoutCert for the next one. A certificate
// for a view the pacemaker already left is stale.
func (pacemaker *Pacemaker) AdvanceRound(timeoutCert finality.TimeoutCert) error {
	if timeoutCert.Round < pacemaker.round || timeoutCert.Round == ^types.Round(0) {
		return ErrStaleTimeoutCert
	}
	pacemaker.round = timeoutCert.Round + 1
	return nil
}

// AdvanceHeight raises the height. The view is unaffected.
func (pacemaker *Pacemaker) AdvanceHeight(height types.Height) {
	if height > pacemaker.height {
		pacemaker.height = height
	}
}

// Advance raises the height and the view to at least the given values and reports
// whether either moved.
func (pacemaker *Pacemaker) Advance(height types.Height, round types.Round) bool {
	moved := false
	if height > pacemaker.height {
		pacemaker.height = height
		moved = true
	}
	if round > pacemaker.round {
		pacemaker.round = round
		moved = true
	}
	return moved
}
