// Package registry provides a concurrency-safe name-keyed store that rejects
// duplicate keys. It is shared by the notification integrations in this module.
package registry

import (
	"errors"
	"sync"
)

// ErrExists is returned by Register when the key is already present.
var ErrExists = errors.New("registry: key already exists")

// Registry is a concurrency-safe map of keys to values of type T.
type Registry[T any] struct {
	mu sync.RWMutex
	m  map[string]T
}

// New returns an empty Registry.
func New[T any]() *Registry[T] {
	return &Registry[T]{m: make(map[string]T)}
}

// Register stores value under key. It returns ErrExists if the key is taken.
func (r *Registry[T]) Register(key string, value T) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.m[key]; exists {
		return ErrExists
	}
	r.m[key] = value
	return nil
}

// Get returns the value stored under key.
func (r *Registry[T]) Get(key string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, exists := r.m[key]
	return value, exists
}

// GetOrCreate returns the value stored under key, creating and storing one via
// create when the key is absent. create is called at most once per key.
func (r *Registry[T]) GetOrCreate(key string, create func() T) T {
	r.mu.Lock()
	defer r.mu.Unlock()
	if value, exists := r.m[key]; exists {
		return value
	}
	value := create()
	r.m[key] = value
	return value
}

// Delete removes the value stored under key.
func (r *Registry[T]) Delete(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, key)
}

// Reset empties the registry.
func (r *Registry[T]) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = make(map[string]T)
}
