package mutex_map

import "sync"

type MutexMap[K comparable, V any] struct {
	mutex  sync.RWMutex
	values map[K]V
}

func New[K comparable, V any]() *MutexMap[K, V] {
	return &MutexMap[K, V]{
		mutex:  sync.RWMutex{},
		values: make(map[K]V),
	}
}

func (m *MutexMap[K, V]) Get(key K) V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.values[key]
}

func (m *MutexMap[K, V]) Set(key K, val V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.values[key] = val
}

func (m *MutexMap[K, V]) Delete(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.values, key)
}
