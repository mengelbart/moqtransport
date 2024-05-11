package moqtransport

import (
	"errors"
	"sync"
)

type syncMap[K comparable, V any] struct {
	mutex    sync.Mutex
	elements map[K]V
}

func newSyncMap[K comparable, V any]() *syncMap[K, V] {
	return &syncMap[K, V]{
		mutex:    sync.Mutex{},
		elements: make(map[K]V),
	}
}

func (m *syncMap[K, V]) add(k K, v V) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.elements[k]; ok {
		return errors.New("duplicate entry")
	}
	m.elements[k] = v
	return nil
}

func (m *syncMap[K, V]) get(k K) (V, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	v, ok := m.elements[k]
	return v, ok
}

func (m *syncMap[K, V]) delete(k K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.elements, k)
}
