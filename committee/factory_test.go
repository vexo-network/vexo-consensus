package committee

import (
	"errors"
	"testing"
)

func TestNewSelectorDeterministic(t *testing.T) {
	selector, err := NewSelector(RotationPolicy{Backend: BackendDeterministic, EpochLength: 1, CommitteeSize: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selector == nil {
		t.Fatal("expected selector")
	}
}

func TestNewSelectorVRFRequiresVRF(t *testing.T) {
	_, err := NewSelector(RotationPolicy{Backend: BackendVRF, EpochLength: 1, CommitteeSize: 1}, nil)
	if !errors.Is(err, ErrMissingVRF) {
		t.Fatalf("expected missing vrf, got %v", err)
	}
}

func TestNewSelectorRejectsUnsupportedBackend(t *testing.T) {
	_, err := NewSelector(RotationPolicy{Backend: "unknown", EpochLength: 1, CommitteeSize: 1}, nil)
	if !errors.Is(err, ErrUnsupportedCommitteeBackend) {
		t.Fatalf("expected unsupported backend, got %v", err)
	}
}
