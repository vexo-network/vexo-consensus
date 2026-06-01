package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrStaleTimeoutCert = errors.New("stale timeout certificate")

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

func (pacemaker *Pacemaker) AdvanceRound(timeoutCert finality.TimeoutCert) error {
	if timeoutCert.Height != pacemaker.height || timeoutCert.Round < pacemaker.round {
		return ErrStaleTimeoutCert
	}
	pacemaker.round = timeoutCert.Round + 1
	return nil
}

func (pacemaker *Pacemaker) AdvanceHeight(height types.Height) {
	if height > pacemaker.height {
		pacemaker.height = height
		pacemaker.round = 0
	}
}
