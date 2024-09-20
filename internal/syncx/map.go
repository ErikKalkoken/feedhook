package syncx

import "sync"

// Map represents a generic hashmap that is safe to use concurrently.
type Map[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewMap returns a new Map.
func NewMap[K comparable, V any]() *Map[K, V] {
	sm := &Map[K, V]{
		m: make(map[K]V),
	}
	return sm
}

// Load returns the value stored in the map for a key, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (sm *Map[K, V]) Load(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

// Store sets the value for a key.
func (sm *Map[K, V]) Store(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}
