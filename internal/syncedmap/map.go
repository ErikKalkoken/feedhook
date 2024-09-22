package syncedmap

import "sync"

// SyncedMap represents a generic hashmap that is safe to use concurrently.
type SyncedMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// New returns a new Map.
func New[K comparable, V any]() *SyncedMap[K, V] {
	sm := &SyncedMap[K, V]{
		m: make(map[K]V),
	}
	return sm
}

// Load returns the value stored in the map for a key, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (sm *SyncedMap[K, V]) Load(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

// Store sets the value for a key.
func (sm *SyncedMap[K, V]) Store(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}
