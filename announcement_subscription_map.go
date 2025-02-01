package moqtransport

import (
	"slices"
	"sync"
)

type announcementSubscriptionMap struct {
	lock sync.Mutex
	as   []announcementSubscription
}

func newAnnouncementSubscriptionMap() *announcementSubscriptionMap {
	return &announcementSubscriptionMap{
		lock: sync.Mutex{},
		as:   []announcementSubscription{},
	}
}

// add returns an error if the entry is already present
func (m *announcementSubscriptionMap) add(a announcementSubscription) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as announcementSubscription) bool {
		return slices.Equal(a.namespace, as.namespace)
	})
	if i >= 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.as = append(m.as, a)
	return nil
}

func (m *announcementSubscriptionMap) get(namespace []string) (announcementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as announcementSubscription) bool {
		return slices.Equal(as.namespace, namespace)
	})
	if i < 0 {
		return announcementSubscription{}, false
	}
	return m.as[i], true
}

// delete returns the deleted element (if present) and whether the entry was
// present and removed.
func (m *announcementSubscriptionMap) delete(namespace []string) (announcementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as announcementSubscription) bool {
		return slices.Equal(as.namespace, namespace)
	})
	if i < 0 {
		return announcementSubscription{}, false
	}
	e := m.as[i]
	m.as = append(m.as[:i], m.as[i+1:]...)
	return e, true
}
