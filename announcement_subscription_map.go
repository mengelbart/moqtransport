package moqtransport

import (
	"slices"
	"sync"
)

type announcementSubscriptionMap struct {
	lock sync.Mutex
	as   map[uint64]*announcementSubscription
}

func newAnnouncementSubscriptionMap() *announcementSubscriptionMap {
	return &announcementSubscriptionMap{
		lock: sync.Mutex{},
		as:   make(map[uint64]*announcementSubscription),
	}
}

func findAnnouncementSubscription(as map[uint64]*announcementSubscription, namespace []string) *announcementSubscription {
	for _, v := range as {
		if slices.Equal(namespace, v.namespace) {
			return v
		}
	}
	return nil
}

// add returns an error if the entry is already present
func (m *announcementSubscriptionMap) add(a *announcementSubscription) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.as[a.requestID] = a
	return nil
}

// delete returns the deleted element (if present) and whether the entry was
// present and removed.
func (m *announcementSubscriptionMap) delete(namespace []string) (*announcementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	as := findAnnouncementSubscription(m.as, namespace)
	if as != nil {
		delete(m.as, as.requestID)
		return as, true
	}
	return nil, false
}

func (m *announcementSubscriptionMap) deleteByID(requestID uint64) (*announcementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	as, ok := m.as[requestID]
	delete(m.as, requestID)
	return as, ok
}
