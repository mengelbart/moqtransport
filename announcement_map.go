package moqtransport

import "sync"

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

func (m *announcementMap) add(a Announcement) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pending = append(m.pending, a)
}
