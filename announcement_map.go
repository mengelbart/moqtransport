package moqtransport

import (
	"slices"
	"sync"
)

type announcementMap struct {
	lock          sync.Mutex
	pending       []*Announcement
	announcements []*Announcement
}

func newAnnouncementMap() *announcementMap {
	return &announcementMap{
		lock:    sync.Mutex{},
		pending: []*Announcement{},
	}
}

func find(as []*Announcement, namespace []string) int {
	return slices.IndexFunc(as, func(x *Announcement) bool {
		return slices.Equal(namespace, x.Namespace)
	})
}

func (m *announcementMap) add(a *Announcement) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := find(m.pending, a.Namespace)
	if i >= 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.pending = append(m.pending, a)
	return nil
}

func (m *announcementMap) confirmAndGet(namespace []string) (*Announcement, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := find(m.pending, namespace)
	if i < 0 {
		return nil, errUnknownAnnouncement
	}
	e := m.pending[i]
	m.pending = append(m.pending[:i], m.pending[i+1:]...)
	i = find(m.announcements, e.Namespace)
	if i > 0 {
		return nil, errDuplicateAnnouncementNamespace
	}
	m.announcements = append(m.announcements, e)
	return e, nil
}

func (m *announcementMap) confirm(a *Announcement) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := find(m.pending, a.Namespace)
	if i < 0 {
		return errUnknownAnnouncement
	}
	e := m.pending[i]
	m.pending = append(m.pending[:i], m.pending[i+1:]...)

	i = find(m.announcements, e.Namespace)
	if i > 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.announcements = append(m.announcements, e)
	return nil
}

func (m *announcementMap) reject(namespace []string) (*Announcement, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := find(m.pending, namespace)
	if i < 0 {
		return nil, false
	}
	e := m.pending[i]
	m.pending = append(m.pending[:i], m.pending[i+1:]...)
	return e, true
}
