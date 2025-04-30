package moqtransport

import (
	"slices"
	"sync"
)

func findAnnouncement(as map[uint64]*announcement, namespace []string) *announcement {
	for _, v := range as {
		if slices.Equal(namespace, v.namespace) {
			return v
		}
	}
	return nil
}

type announcementMap struct {
	lock          sync.Mutex
	pending       map[uint64]*announcement
	announcements map[uint64]*announcement
}

func newAnnouncementMap() *announcementMap {
	return &announcementMap{
		lock:          sync.Mutex{},
		pending:       map[uint64]*announcement{},
		announcements: map[uint64]*announcement{},
	}
}

func (m *announcementMap) add(a *announcement) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pending[a.requestID] = a
	return nil
}

func (m *announcementMap) confirmAndGet(rid uint64) (*announcement, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	a, ok := m.pending[rid]
	if !ok {
		return nil, errUnknownAnnouncement
	}
	delete(m.pending, rid)
	m.announcements[rid] = a
	return a, nil
}

func (m *announcementMap) reject(rid uint64) (*announcement, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	a, ok := m.pending[rid]
	if !ok {
		return nil, false
	}
	delete(m.pending, rid)
	return a, true
}

func (m *announcementMap) delete(namespace []string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	a := findAnnouncement(m.pending, namespace)
	if a != nil {
		delete(m.pending, a.requestID)
		return true
	}
	a = findAnnouncement(m.announcements, namespace)
	if a != nil {
		delete(m.announcements, a.requestID)
		return true
	}
	return false
}
