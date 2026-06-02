package crypto

import "errors"

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
	for id, record := range keyRing.keys {
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
