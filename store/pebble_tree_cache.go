package store

import (
	"bytes"
	"sync"

	"github.com/cockroachdb/pebble/v2"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
)

// Tree node cache.
//
// Computing a root or committing a block reads every node on the path of each key it writes, and
// the runtime computes the roots of a block's writes for the app hash and the state roots before
// it commits them. The nodes near a root are on nearly every path, so every block reads them again,
// from SSTables that compactions keep rewriting. The store therefore keeps the encoded nodes of the
// current trees in memory, decoded, so a read neither decodes nor allocates.
//
// The cache holds the stored node of a node key as of the current state, and only nodes that
// exist. Every batch the store commits goes through writeBatch, which applies the batch's sets and
// deletes of tree nodes to the cache, so the cache never holds a node the store no longer has, and
// drops the nodes the batch orphans (see State trees), so it holds the nodes of the current trees
// rather than those of the heights before. A
// generation orders the cache against snapshots: it is odd while a batch that changes tree nodes
// commits, and a reader uses the cache only at the even generation it saw both before and after it
// opened its snapshot, so it never mixes nodes of two states.

// treeNodeCacheCapacity bounds the nodes the cache holds. A node is admitted while the cache has
// room, so the nodes near the roots, which every block reads first, stay; the cache holds at most
// about 50 MiB.
const treeNodeCacheCapacity = 1 << 18

type treeNodeCache struct {
	mu         sync.Mutex
	generation uint64
	nodes      map[string]storedNode
}

// storedNode is a decoded stored node and the versions of its halves. The leaf a node points to is
// shared by every reader of the node and never modified.
type storedNode struct {
	node        stateproof.Node
	left, right types.Height
}

func newTreeNodeCache() *treeNodeCache {
	return &treeNodeCache{nodes: make(map[string]storedNode)}
}

// treeCacheView is the cache as one reader may use it: at generation, or not at all.
type treeCacheView struct {
	cache      *treeNodeCache
	generation uint64
}

// stable returns the current generation, and whether no batch that changes tree nodes is
// committing.
func (cache *treeNodeCache) stable() (uint64, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.generation, cache.generation%2 == 0
}

// get returns the node at key when the cache is still at the view's generation and holds it.
func (view treeCacheView) get(key []byte) (storedNode, bool) {
	if view.cache == nil {
		return storedNode{}, false
	}
	view.cache.mu.Lock()
	defer view.cache.mu.Unlock()
	if view.cache.generation != view.generation {
		return storedNode{}, false
	}
	stored, found := view.cache.nodes[string(key)]
	return stored, found
}

// add caches the node a reader read at key at the view's generation.
func (view treeCacheView) add(key []byte, stored storedNode) {
	if view.cache == nil {
		return
	}
	view.cache.mu.Lock()
	defer view.cache.mu.Unlock()
	if view.cache.generation != view.generation || len(view.cache.nodes) >= treeNodeCacheCapacity {
		return
	}
	view.cache.nodes[string(key)] = stored
}

// treeNodeWrite is a set (value non-nil) or delete of a tree node key in a batch.
type treeNodeWrite struct {
	key   []byte
	value []byte
}

// treeNodeWrites returns the writes of batch to tree node keys, in batch order.
func treeNodeWrites(batch *pebble.Batch) ([]treeNodeWrite, error) {
	var writes []treeNodeWrite
	reader := batch.Reader()
	for {
		kind, key, value, ok, err := reader.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return writes, nil
		}
		if kind == pebble.InternalKeyKindSet && bytes.HasPrefix(key, stateTreeOrphanPrefix) {
			if err := eachListedKey(value, func(entry []byte) {
				writes = append(writes, treeNodeWrite{key: append(bytes.Clone(stateTreePrefix), entry...)})
			}); err != nil {
				return nil, err
			}
			continue
		}
		if !bytes.HasPrefix(key, stateTreePrefix) {
			continue
		}
		switch kind {
		case pebble.InternalKeyKindSet:
			writes = append(writes, treeNodeWrite{key: key, value: value})
		case pebble.InternalKeyKindDelete, pebble.InternalKeyKindSingleDelete:
			writes = append(writes, treeNodeWrite{key: key})
		default:
			// The store writes tree nodes only with sets and deletes; anything else drops the cache.
			writes = append(writes, treeNodeWrite{})
		}
	}
}

// commit commits batch with options and applies its tree node writes to the cache.
func (cache *treeNodeCache) commit(batch *pebble.Batch, options *pebble.WriteOptions) error {
	writes, err := treeNodeWrites(batch)
	if err != nil {
		return err
	}
	if len(writes) == 0 {
		return batch.Commit(options)
	}
	cache.mu.Lock()
	cache.generation++
	cache.mu.Unlock()
	err = batch.Commit(options)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.generation++
	if err != nil {
		// Whether the batch applied is unknown; nothing cached can be trusted.
		cache.nodes = make(map[string]storedNode)
		return err
	}
	for _, write := range writes {
		switch {
		case write.key == nil:
			cache.nodes = make(map[string]storedNode)
		case write.value == nil:
			delete(cache.nodes, string(write.key))
		default:
			if _, held := cache.nodes[string(write.key)]; !held && len(cache.nodes) >= treeNodeCacheCapacity {
				continue
			}
			stored, err := decodeStoredNode(write.value)
			if err != nil {
				// A node that does not decode is read, and refused, from the database.
				delete(cache.nodes, string(write.key))
				continue
			}
			cache.nodes[string(write.key)] = stored
		}
	}
	return nil
}
