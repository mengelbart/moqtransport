package moqtransport

import (
	"errors"
	"slices"
	"sync"
)

var errDuplicateEntry = errors.New("entry already present")

type announcementSubscriptionMap struct {
	lock sync.Mutex
	as   []AnnouncementSubscription
}

func newAnnouncementSubscriptionMap() *announcementSubscriptionMap {
	return &announcementSubscriptionMap{
		lock: sync.Mutex{},
		as:   []AnnouncementSubscription{},
	}
}

// add returns an error if the entry is already present
func (m *announcementSubscriptionMap) add(a AnnouncementSubscription) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as AnnouncementSubscription) bool {
		return slices.Equal(a.namespace, as.namespace)
	})
	if i >= 0 {
		return errDuplicateEntry
	}
	m.as = append(m.as, a)
	return nil
}

func (m *announcementSubscriptionMap) get(namespace []string) (AnnouncementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as AnnouncementSubscription) bool {
		return slices.Equal(as.namespace, namespace)
	})
	if i < 0 {
		return AnnouncementSubscription{}, false
	}
	return m.as[i], true
}

// delete returns the deleted element (if present) and whether the entry was
// present and removed.
func (m *announcementSubscriptionMap) delete(namespace []string) (AnnouncementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.as, func(as AnnouncementSubscription) bool {
		return slices.Equal(as.namespace, namespace)
	})
	if i < 0 {
		return AnnouncementSubscription{}, false
	}
	e := m.as[i]
	m.as = append(m.as[:i], m.as[i+1:]...)
	return e, true
}
