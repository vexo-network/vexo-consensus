package committee

import "errors"

var ErrUnsupportedCommitteeBackend = errors.New("unsupported committee backend")

func NewSelector(policy RotationPolicy, vrf VRF) (Selector, error) {
	switch policy.Backend {
	case "", BackendDeterministic:
		return NewDeterministicSelector(policy)
	case BackendVRF:
		return NewVRFSelector(policy, vrf)
	default:
		return nil, ErrUnsupportedCommitteeBackend
	}
}
