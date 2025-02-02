package moqtransport

import (
	"slices"
	"sync"
)

type announcementMap struct {
	lock          sync.Mutex
	pending       []*announcement
	announcements []*announcement
}

func newAnnouncementMap() *announcementMap {
	return &announcementMap{
		lock:    sync.Mutex{},
		pending: []*announcement{},
	}
}

func findAnnouncement(as []*announcement, namespace []string) int {
	return slices.IndexFunc(as, func(x *announcement) bool {
		return slices.Equal(namespace, x.Namespace)
	})
}

func (m *announcementMap) add(a *announcement) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncement(m.pending, a.Namespace)
	if i >= 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.pending = append(m.pending, a)
	return nil
}

func (m *announcementMap) confirmAndGet(namespace []string) (*announcement, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncement(m.pending, namespace)
	if i < 0 {
		return nil, errUnknownAnnouncement
	}
	e := m.pending[i]
	m.pending = slices.Delete(m.pending, i, i+1)
	i = findAnnouncement(m.announcements, e.Namespace)
	if i > 0 {
		return nil, errDuplicateAnnouncementNamespace
	}
	m.announcements = append(m.announcements, e)
	return e, nil
}

func (m *announcementMap) confirm(namespace []string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncement(m.pending, namespace)
	if i < 0 {
		return errUnknownAnnouncement
	}
	e := m.pending[i]
	m.pending = slices.Delete(m.pending, i, i+1)

	i = findAnnouncement(m.announcements, e.Namespace)
	if i > 0 {
		return errDuplicateAnnouncementNamespace
	}
	m.announcements = append(m.announcements, e)
	return nil
}

func (m *announcementMap) reject(namespace []string) (*announcement, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	i := findAnnouncement(m.pending, namespace)
	if i < 0 {
		return nil, false
	}
	e := m.pending[i]
	m.pending = slices.Delete(m.pending, i, i+1)
	return e, true
}

func (m *announcementMap) delete(namespace []string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	deleted := false
	i := findAnnouncement(m.pending, namespace)
	if i >= 0 {
		m.pending = slices.Delete(m.pending, i, i+1)
		deleted = true
	}
	i = findAnnouncement(m.announcements, namespace)
	if i < 0 {
		m.announcements = slices.Delete(m.announcements, i, i+1)
		deleted = true
	}
	return deleted
}
