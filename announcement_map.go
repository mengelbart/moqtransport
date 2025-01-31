package moqtransport

import (
	"slices"
	"sync"
)

type announcementMap struct {
	lock    sync.Mutex
	pending []Announcement
}

func newAnnouncementMap() *announcementMap {
	return &announcementMap{
		lock:    sync.Mutex{},
		pending: []Announcement{},
	}
}

func (m *announcementMap) add(a Announcement) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.pending, func(as Announcement) bool {
		return slices.Equal(a.Namespace, as.Namespace)
	})
	if i >= 0 {
		return errDuplicateEntry
	}
	m.pending = append(m.pending, a)
	return nil
}

func (m *announcementMap) delete(namespace []string) (Announcement, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := slices.IndexFunc(m.pending, func(as Announcement) bool {
		return slices.Equal(namespace, as.Namespace)
	})
	if i < 0 {
		return Announcement{}, false
	}
	e := m.pending[i]
	m.pending = append(m.pending[:i], m.pending[i+1:]...)
	return e, true
}
