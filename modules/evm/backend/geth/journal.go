package geth

import (
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// The state adapter keeps an undo journal instead of copying state on every
// Snapshot. A snapshot is the journal length at the time it was taken and a
// revert replays the inverse of every later mutation, so the cost of both is
// proportional to the mutations performed inside the reverted frame rather than
// to everything the transaction touched so far.
//
// Storage values loaded from the reader are cached for the lifetime of the
// execution and are not journaled: they equal the committed state, which does
// not change while a transaction executes.

type journalKind uint8

const (
	journalBalance journalKind = iota + 1
	journalNonce
	journalCode
	journalStorage
	journalTransient
	journalSelfDestruct
	journalNewContract
	journalTouch
	journalRefund
	journalLogs
	journalAccessAddress
	journalAccessSlot
)

// journalEntry is the inverse of one mutation. Which fields are meaningful
// depends on kind: word holds the previous storage, transient or balance
// word, number the previous nonce, refund or log count, flag the previous
// boolean marker and code the previous contract code.
type journalEntry struct {
	kind    journalKind
	flag    bool
	address gethcommon.Address
	slot    gethcommon.Hash
	word    [32]byte
	number  uint64
	code    []byte
}

func (db *gethStateDB) journalAppend(entry journalEntry) {
	db.journal = append(db.journal, entry)
}

func (db *gethStateDB) journalBalance(address gethcommon.Address, previous uint256.Int) {
	db.journalAppend(journalEntry{kind: journalBalance, address: address, word: previous.Bytes32()})
}

// revertJournal undoes every mutation recorded after the journal had the given
// length, newest first.
func (db *gethStateDB) revertJournal(length int) {
	for index := len(db.journal) - 1; index >= length; index-- {
		db.revertEntry(db.journal[index])
		db.journal[index] = journalEntry{}
	}
	db.journal = db.journal[:length]
}

func (db *gethStateDB) revertEntry(entry journalEntry) {
	switch entry.kind {
	case journalBalance:
		db.accounts[entry.address].balance.SetBytes32(entry.word[:])
	case journalNonce:
		db.accounts[entry.address].nonce = entry.number
	case journalCode:
		db.accounts[entry.address].code = entry.code
	case journalStorage:
		db.accounts[entry.address].storage[entry.slot] = gethcommon.Hash(entry.word)
	case journalTransient:
		db.accounts[entry.address].transient[entry.slot] = gethcommon.Hash(entry.word)
	case journalSelfDestruct:
		db.accounts[entry.address].selfDestructed = entry.flag
	case journalNewContract:
		db.accounts[entry.address].newContract = entry.flag
	case journalTouch:
		db.accounts[entry.address].touched = entry.flag
	case journalRefund:
		db.refund = entry.number
	case journalLogs:
		clear(db.logs[entry.number:])
		db.logs = db.logs[:entry.number]
	case journalAccessAddress:
		delete(db.accessList, entry.address)
	case journalAccessSlot:
		delete(db.accessList[entry.address], entry.slot)
	}
}
