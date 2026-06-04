package crypto

import (
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrKeyAlreadyExists = errors.New("key already exists")
	ErrEmptyKeyID       = errors.New("key id is empty")
	ErrNilSigner        = errors.New("signer is nil")
	ErrInactiveKey      = errors.New("key is not active at height")
	ErrInvalidKeyWindow = errors.New("invalid key activation window")
)

type KeyID string

type KeyRecord struct {
	ID          KeyID
	Signer      Signer
	ActiveFrom  uint64
	ActiveUntil uint64
}

type KeyRing struct {
	keys   map[KeyID]KeyRecord
	active KeyID
}

type KeyRingPolicySigner struct {
	keyRing *KeyRing
}

func NewKeyRing(records ...KeyRecord) (*KeyRing, error) {
	keyRing := &KeyRing{keys: make(map[KeyID]KeyRecord)}
	for _, record := range records {
		if err := keyRing.Add(record); err != nil {
			return nil, err
		}
		if keyRing.active == "" {
			keyRing.active = record.ID
		}
	}
	return keyRing, nil
}

func NewKeyRingPolicySigner(records ...KeyRecord) (KeyRingPolicySigner, error) {
	keyRing, err := NewKeyRing(records...)
	if err != nil {
		return KeyRingPolicySigner{}, err
	}
	return KeyRingPolicySigner{keyRing: keyRing}, nil
}

func NewKeyRingPolicySignerFromDocuments(passphrase string, documents ...KeyDocument) (KeyRingPolicySigner, error) {
	records := make([]KeyRecord, 0, len(documents))
	for _, document := range documents {
		record, err := document.KeyRecordWithPassphrase(passphrase)
		if err != nil {
			return KeyRingPolicySigner{}, err
		}
		records = append(records, record)
	}
	return NewKeyRingPolicySigner(records...)
}

func (keyRing *KeyRing) Add(record KeyRecord) error {
	if record.ID == "" {
		return ErrEmptyKeyID
	}
	if record.Signer == nil {
		return ErrNilSigner
	}
	if record.ActiveUntil > 0 && record.ActiveUntil < record.ActiveFrom {
		return ErrInvalidKeyWindow
	}
	if _, exists := keyRing.keys[record.ID]; exists {
		return ErrKeyAlreadyExists
	}
	keyRing.keys[record.ID] = record
	return nil
}

func (keyRing *KeyRing) Activate(id KeyID) error {
	if _, exists := keyRing.keys[id]; !exists {
		return ErrKeyNotFound
	}
	keyRing.active = id
	return nil
}

func (keyRing *KeyRing) ActiveKeyID() KeyID {
	return keyRing.active
}

func (keyRing *KeyRing) ActiveSigner() (Signer, error) {
	return keyRing.Signer(keyRing.active)
}

func (keyRing *KeyRing) ActiveSignerAt(height uint64) (Signer, KeyID, error) {
	if keyRing.active != "" {
		record, exists := keyRing.keys[keyRing.active]
		if exists && record.activeAt(height) {
			return record.Signer, record.ID, nil
		}
	}
	ids := make([]string, 0, len(keyRing.keys))
	for id := range keyRing.keys {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, rawID := range ids {
		id := KeyID(rawID)
		record := keyRing.keys[id]
		if record.activeAt(height) {
			return record.Signer, id, nil
		}
	}
	return nil, "", ErrInactiveKey
}

func (keyRing *KeyRing) Signer(id KeyID) (Signer, error) {
	record, exists := keyRing.keys[id]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return record.Signer, nil
}

func (keyRing *KeyRing) Record(id KeyID) (KeyRecord, error) {
	record, exists := keyRing.keys[id]
	if !exists {
		return KeyRecord{}, ErrKeyNotFound
	}
	return record, nil
}

func (record KeyRecord) activeAt(height uint64) bool {
	if height < record.ActiveFrom {
		return false
	}
	return record.ActiveUntil == 0 || height <= record.ActiveUntil
}

func (signer KeyRingPolicySigner) PublicKey() types.PublicKey {
	activeSigner, err := signer.keyRing.ActiveSigner()
	if err != nil {
		return nil
	}
	return activeSigner.PublicKey()
}

func (signer KeyRingPolicySigner) PublicKeyAt(height uint64) (types.PublicKey, KeyID, error) {
	activeSigner, keyID, err := signer.keyRing.ActiveSignerAt(height)
	if err != nil {
		return nil, "", err
	}
	return activeSigner.PublicKey(), keyID, nil
}

func (signer KeyRingPolicySigner) Sign(message []byte) (types.Signature, error) {
	activeSigner, err := signer.keyRing.ActiveSigner()
	if err != nil {
		return nil, err
	}
	return activeSigner.Sign(message)
}

func (signer KeyRingPolicySigner) SignWithPolicy(policy SignPolicy, message []byte) (types.Signature, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	activeSigner, _, err := signer.keyRing.ActiveSignerAt(uint64(policy.Height))
	if err != nil {
		return nil, err
	}
	if policySigner, ok := activeSigner.(PolicySigner); ok {
		return policySigner.SignWithPolicy(policy, message)
	}
	return activeSigner.Sign(message)
}

func (signer KeyRingPolicySigner) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	for _, record := range signer.keyRing.keys {
		if record.Signer.Verify(publicKey, message, signature) {
			return true
		}
	}
	return false
}
