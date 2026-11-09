package rpc

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

type web3FilterStore struct {
	mu             sync.Mutex
	nextID         uint64
	max            int
	order          []string
	filters        map[string]web3Filter
	onChange       func(Web3FilterStoreSnapshot) error
	lastPersistErr error
}

type Web3FilterStoreSnapshot struct {
	NextID  uint64       `json:"next_id"`
	Max     int          `json:"max"`
	Order   []string     `json:"order"`
	Filters []web3Filter `json:"filters"`
}

type web3Filter struct {
	ID          string `json:"id,omitempty"`
	Type        string
	Address     string
	Addresses   []string
	Topics      [][]string
	FromBlock   uint64
	ToBlock     uint64
	LastHeight  uint64
	SeenPending map[string]bool
	SeenLogs    map[string]bool
}

func newWeb3FilterStore() *web3FilterStore {
	return &web3FilterStore{max: defaultMaxWeb3Filters, filters: make(map[string]web3Filter)}
}

func newWeb3FilterStoreWithConfig(cfg Config) (*web3FilterStore, error) {
	filters := newWeb3FilterStore()
	snapshotPath := strings.TrimSpace(cfg.Web3FilterSnapshotPath)
	if snapshotPath == "" {
		return filters, nil
	}
	filters.setOnChange(func(snapshot Web3FilterStoreSnapshot) error {
		return saveWeb3FilterStoreSnapshotAtomic(snapshotPath, snapshot)
	})
	snapshot, err := loadWeb3FilterStoreSnapshot(snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return filters, nil
	}
	if err != nil {
		return filters, err
	}
	filters.Restore(snapshot)
	return filters, nil
}

func loadWeb3FilterStoreSnapshot(path string) (Web3FilterStoreSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Web3FilterStoreSnapshot{}, err
	}
	var snapshot Web3FilterStoreSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Web3FilterStoreSnapshot{}, err
	}
	return snapshot, nil
}

func saveWeb3FilterStoreSnapshotAtomic(path string, snapshot Web3FilterStoreSnapshot) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

func (store *web3FilterStore) addLog(filter web3Filter, logs []any, latestHeight uint64) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	filter.Type = "log"
	filter.LastHeight = latestHeight
	filter.SeenLogs = web3SeenLogSet(logs)
	store.addLocked(id, filter)
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) addBlock(latestHeight uint64) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	store.addLocked(id, web3Filter{Type: "block", LastHeight: latestHeight})
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) addPending(hashes []types.Hash) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	seen := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		seen[web3HashString(hash)] = true
	}
	store.addLocked(id, web3Filter{Type: "pending", SeenPending: seen})
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) get(id string) (web3Filter, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	filter, found := store.filters[id]
	return filter, found
}

func (store *web3FilterStore) mark(id string, height uint64) {
	store.mu.Lock()
	filter, found := store.filters[id]
	if !found {
		store.mu.Unlock()
		return
	}
	filter.LastHeight = height
	store.filters[id] = filter
	store.mu.Unlock()
	store.persistAfterChange()
}

func (store *web3FilterStore) replace(id string, filter web3Filter) {
	store.mu.Lock()
	if _, found := store.filters[id]; !found {
		store.mu.Unlock()
		return
	}
	store.filters[id] = filter
	store.mu.Unlock()
	store.persistAfterChange()
}

func (store *web3FilterStore) remove(id string) bool {
	store.mu.Lock()
	if _, found := store.filters[id]; !found {
		store.mu.Unlock()
		return false
	}
	store.removeLocked(id)
	store.mu.Unlock()
	store.persistAfterChange()
	return true
}

func (store *web3FilterStore) addLocked(id string, filter web3Filter) {
	if _, found := store.filters[id]; !found {
		store.order = append(store.order, id)
	}
	store.filters[id] = filter
	limit := store.max
	if limit <= 0 {
		limit = defaultMaxWeb3Filters
	}
	for len(store.filters) > limit && len(store.order) > 0 {
		store.removeLocked(store.order[0])
	}
}

func (store *web3FilterStore) removeLocked(id string) {
	delete(store.filters, id)
	for index, current := range store.order {
		if current == id {
			store.order = append(store.order[:index], store.order[index+1:]...)
			return
		}
	}
}

func (store *web3FilterStore) Snapshot() Web3FilterStoreSnapshot {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.snapshotLocked()
}

func (store *web3FilterStore) snapshotLocked() Web3FilterStoreSnapshot {
	filters := make([]web3Filter, 0, len(store.order))
	for _, id := range store.order {
		filter, found := store.filters[id]
		if !found {
			continue
		}
		filter.ID = id
		filter.Addresses = append([]string(nil), filter.Addresses...)
		filter.Topics = cloneWeb3Topics(filter.Topics)
		filter.SeenPending = cloneStringBoolMap(filter.SeenPending)
		filter.SeenLogs = cloneStringBoolMap(filter.SeenLogs)
		filters = append(filters, filter)
	}
	return Web3FilterStoreSnapshot{
		NextID:  store.nextID,
		Max:     store.max,
		Order:   append([]string(nil), store.order...),
		Filters: filters,
	}
}

func (store *web3FilterStore) setOnChange(callback func(Web3FilterStoreSnapshot) error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.onChange = callback
}

func (store *web3FilterStore) persistAfterChange() {
	store.mu.Lock()
	callback := store.onChange
	if callback == nil {
		store.mu.Unlock()
		return
	}
	snapshot := store.snapshotLocked()
	store.mu.Unlock()

	err := callback(snapshot)
	store.mu.Lock()
	store.lastPersistErr = err
	store.mu.Unlock()
}

func (store *web3FilterStore) persistError() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.lastPersistErr
}

func web3FilterPersistRPCError(store *web3FilterStore) *JSONRPCError {
	if store == nil {
		return nil
	}
	if err := store.persistError(); err != nil {
		return &JSONRPCError{Code: -32000, Message: "web3 filter snapshot persistence failed: " + err.Error()}
	}
	return nil
}

func (store *web3FilterStore) Restore(snapshot Web3FilterStoreSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.nextID = snapshot.NextID
	store.max = snapshot.Max
	store.order = make([]string, 0, len(snapshot.Filters))
	store.filters = make(map[string]web3Filter, len(snapshot.Filters))
	for _, filter := range snapshot.Filters {
		id := filter.ID
		if id == "" {
			continue
		}
		filter.ID = ""
		filter.Addresses = append([]string(nil), filter.Addresses...)
		filter.Topics = cloneWeb3Topics(filter.Topics)
		filter.SeenPending = cloneStringBoolMap(filter.SeenPending)
		filter.SeenLogs = cloneStringBoolMap(filter.SeenLogs)
		store.order = append(store.order, id)
		store.filters[id] = filter
	}
	if len(snapshot.Order) > 0 {
		ordered := make([]string, 0, len(snapshot.Order))
		seen := make(map[string]struct{}, len(snapshot.Order))
		for _, id := range snapshot.Order {
			if _, found := store.filters[id]; !found {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			ordered = append(ordered, id)
			seen[id] = struct{}{}
		}
		for id := range store.filters {
			if _, found := seen[id]; !found {
				ordered = append(ordered, id)
			}
		}
		store.order = ordered
	}
	limit := store.max
	if limit <= 0 {
		limit = defaultMaxWeb3Filters
	}
	for len(store.filters) > limit && len(store.order) > 0 {
		store.removeLocked(store.order[0])
	}
}

func cloneWeb3Topics(topics [][]string) [][]string {
	if topics == nil {
		return nil
	}
	cloned := make([][]string, len(topics))
	for index, topicGroup := range topics {
		cloned[index] = append([]string(nil), topicGroup...)
	}
	return cloned
}

func cloneStringBoolMap(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
