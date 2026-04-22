package util

import "sync"

// ConcurrentMap 用显式锁保护 map，避免依赖 sync.Map 的内部实现细节。
// 这里的缓存规模都不大，读多写少，RWMutex 足够稳定且行为更直观。
type ConcurrentMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func (m *ConcurrentMap[K, V]) Load(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		var zero V
		return zero, false
	}
	value, ok := m.data[key]
	return value, ok
}

func (m *ConcurrentMap[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[K]V)
	}
	m.data[key] = value
}

func (m *ConcurrentMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		return
	}
	delete(m.data, key)
}

func (m *ConcurrentMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[K]V)
	}
	if current, ok := m.data[key]; ok {
		return current, true
	}
	m.data[key] = value
	return value, false
}
