package moqtransport

import (
	"slices"
	"sync"
)

type announcementSubscriptionMap struct {
	lock sync.Mutex
	as   []*announcementSubscription
}

func newAnnouncementSubscriptionMap() *announcementSubscriptionMap {
	return &announcementSubscriptionMap{
		lock: sync.Mutex{},
		as:   []*announcementSubscription{},
	}
}

func findAnnouncementSubscription(as []*announcementSubscription, namespace []string) int {
	return slices.IndexFunc(as, func(x *announcementSubscription) bool {
		return slices.Equal(namespace, x.namespace)
	})
}

// add returns an error if the entry is already present
func (m *announcementSubscriptionMap) add(a *announcementSubscription) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncementSubscription(m.as, a.namespace)
	if i >= 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.as = append(m.as, a)
	return nil
}

// delete returns the deleted element (if present) and whether the entry was
// present and removed.
func (m *announcementSubscriptionMap) delete(namespace []string) (*announcementSubscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncementSubscription(m.as, namespace)
	if i < 0 {
		return nil, false
	}
	e := m.as[i]
	m.as = slices.Delete(m.as, i, i+1)
	return e, true
}
