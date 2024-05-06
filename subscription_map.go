package moqtransport

import (
	"context"
	"errors"
	"sync"
)

type subscription interface {
	unsubscribe() error
}

type subscriptionMap[T subscription] struct {
	mutex         sync.Mutex
	subscriptions map[uint64]T
	newSubChan    chan uint64
}

func newSubscriptionMap[T subscription]() *subscriptionMap[T] {
	return &subscriptionMap[T]{
		mutex:         sync.Mutex{},
		subscriptions: make(map[uint64]T),
		newSubChan:    make(chan uint64, 1),
	}
}

func (m *subscriptionMap[T]) add(id uint64, t T) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.subscriptions[id]; ok {
		return errors.New("duplicate subscribe id")
	}
	m.subscriptions[id] = t
	m.newSubChan <- id
	return nil
}

func (m *subscriptionMap[T]) get(id uint64) (T, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	sub, ok := m.subscriptions[id]
	return sub, ok
}

func (m *subscriptionMap[T]) delete(id uint64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	sub, ok := m.subscriptions[id]
	delete(m.subscriptions, id)
	if !ok {
		return errors.New("delete on unknown subscribe ID")
	}
	return sub.unsubscribe()
}

func (m *subscriptionMap[T]) getNext(ctx context.Context) (T, error) {
	for {
		select {
		case <-ctx.Done():
			return *new(T), ctx.Err()
		case id := <-m.newSubChan:
			if sub, ok := m.get(id); ok {
				return sub, nil
			}
		}
	}
}
