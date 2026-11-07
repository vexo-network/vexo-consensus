package staking

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

// maxIdentityLength bounds delegator and validator identifiers so a transaction cannot bloat state keys.
const maxIdentityLength = 256

const (
	// stakeKeyPrefix keys a delegation by delegator: a voter's power is one scan of its own prefix.
	stakeKeyPrefix = "stake/"
	// validatorStakeKeyPrefix keys the same delegation by validator, so slashing and fee
	// distribution read the delegations of the validators they act on and no others.
	validatorStakeKeyPrefix  = "validator_stake/"
	validatorKeyPrefix       = "validator/"
	unbondingKeyPrefix       = "unbonding_entries/"
	rewardKeyPrefix          = "rewards/"
	delegationHeightPrefix   = "delegation_height/"
	consensusKeyIndexPrefix  = "consensus_key/"
	slashMarkerKeyPrefix     = "slash/"
	validatorFieldPower      = "power"
	validatorFieldRecord     = "record"
	validatorFieldJail       = "jail"
	validatorFieldTombstone  = "tombstone"
	validatorFieldCommission = "commission_bps"
)

// validateIdentity rejects identifiers that would make the slash-separated state keys ambiguous.
// A "/" in a delegator or validator id lets stake hide from every prefix scan that parses
// stake/<delegator>/<validator>, so such ids can never be written.
func validateIdentity(value string) error {
	if value == "" || len(value) > maxIdentityLength || strings.ContainsAny(value, "/\x00") {
		return ErrInvalidIdentity
	}
	return nil
}

func validateDelegator(delegator types.Address) error {
	return validateIdentity(string(delegator))
}

func validateValidatorID(validatorID types.ValidatorID) error {
	return validateIdentity(string(validatorID))
}

// identityKey is the canonical account identity used inside state keys, so one account can never
// own several ledgers because of address spelling.
func identityKey(address types.Address) string {
	return vexoapp.NormalizeActor(address)
}

func stakeKey(delegator types.Address, validatorID types.ValidatorID) []byte {
	return []byte(stakeKeyPrefix + identityKey(delegator) + "/" + string(validatorID))
}

func validatorStakePrefix(validatorID types.ValidatorID) []byte {
	return []byte(validatorStakeKeyPrefix + string(validatorID) + "/")
}

func validatorStakeKey(validatorID types.ValidatorID, delegator types.Address) []byte {
	return append(validatorStakePrefix(validatorID), identityKey(delegator)...)
}

func validatorFieldKey(validatorID types.ValidatorID, field string) []byte {
	return []byte(validatorKeyPrefix + string(validatorID) + "/" + field)
}

func validatorPowerKey(validatorID types.ValidatorID) []byte {
	return validatorFieldKey(validatorID, validatorFieldPower)
}

func validatorRecordKey(validatorID types.ValidatorID) []byte {
	return validatorFieldKey(validatorID, validatorFieldRecord)
}

func jailKey(validatorID types.ValidatorID) []byte {
	return validatorFieldKey(validatorID, validatorFieldJail)
}

func tombstoneKey(validatorID types.ValidatorID) []byte {
	return validatorFieldKey(validatorID, validatorFieldTombstone)
}

func commissionKey(validatorID types.ValidatorID) []byte {
	return validatorFieldKey(validatorID, validatorFieldCommission)
}

func consensusKeyIndexKey(publicKey []byte) []byte {
	return []byte(consensusKeyIndexPrefix + hex.EncodeToString(publicKey))
}

func delegationHeightKey(delegator types.Address) []byte {
	return []byte(delegationHeightPrefix + identityKey(delegator))
}

func rewardKey(delegator types.Address, validatorID types.ValidatorID) []byte {
	return []byte(rewardKeyPrefix + identityKey(delegator) + "/" + string(validatorID))
}

func slashMarkerKey(offenceKey string) []byte {
	return []byte(slashMarkerKeyPrefix + offenceKey)
}

// unbondingValidatorPrefix holds every unbonding entry of one validator. Entries are keyed by
// validator first, so slashing reads the entries of the validator it punishes and no others, and the
// entries of one delegator/validator pair are still one contiguous prefix.
func unbondingValidatorPrefix(validatorID types.ValidatorID) []byte {
	return []byte(unbondingKeyPrefix + string(validatorID) + "/")
}

func unbondingPairPrefix(delegator types.Address, validatorID types.ValidatorID) []byte {
	return append(unbondingValidatorPrefix(validatorID), []byte(identityKey(delegator)+"/")...)
}

func unbondingEntryKey(delegator types.Address, validatorID types.ValidatorID, created types.Height) []byte {
	return append(unbondingPairPrefix(delegator, validatorID), []byte(fmt.Sprintf("%020d", uint64(created)))...)
}

// parseStakeKey splits stake/<delegator>/<validator>.
func parseStakeKey(key string) (types.Address, types.ValidatorID, bool) {
	rest, found := strings.CutPrefix(key, stakeKeyPrefix)
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return types.Address(parts[0]), types.ValidatorID(parts[1]), true
}

// parseValidatorStakeKey splits validator_stake/<validator>/<delegator>.
func parseValidatorStakeKey(key string) (types.ValidatorID, types.Address, bool) {
	rest, found := strings.CutPrefix(key, validatorStakeKeyPrefix)
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return types.ValidatorID(parts[0]), types.Address(parts[1]), true
}

// parseValidatorKey splits validator/<id>/<field>.
func parseValidatorKey(key string) (types.ValidatorID, string, bool) {
	rest, found := strings.CutPrefix(key, validatorKeyPrefix)
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return types.ValidatorID(parts[0]), parts[1], true
}

// parseUnbondingKey splits unbonding_entries/<validator>/<delegator>/<created height>.
func parseUnbondingKey(key string) (types.Address, types.ValidatorID, types.Height, bool) {
	rest, found := strings.CutPrefix(key, unbondingKeyPrefix)
	if !found {
		return "", "", 0, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", 0, false
	}
	created, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return "", "", 0, false
	}
	return types.Address(parts[1]), types.ValidatorID(parts[0]), types.Height(created), true
}
