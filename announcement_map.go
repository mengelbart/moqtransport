package moqtransport

import (
	"context"
	"errors"
	"sync"
)

type announcementMap struct {
	mutex               sync.Mutex
	announcements       map[string]*Announcement
	newAnnouncementChan chan string
}

func newAnnouncementMap() *announcementMap {
	return &announcementMap{
		mutex:               sync.Mutex{},
		announcements:       map[string]*Announcement{},
		newAnnouncementChan: make(chan string, 1),
	}
}

func (m *announcementMap) add(name string, a *Announcement) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.announcements[name]; ok {
		return errors.New("duplicate announcement")
	}
	m.announcements[name] = a
	m.newAnnouncementChan <- name
	return nil
}

func (m *announcementMap) get(name string) (*Announcement, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	a, ok := m.announcements[name]
	return a, ok
}

func (m *announcementMap) getNext(ctx context.Context) (*Announcement, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case id := <-m.newAnnouncementChan:
			if sub, ok := m.get(id); ok {
				return sub, nil
			}
		}
	}
}
