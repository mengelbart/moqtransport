package main

import (
	"strings"
	"sync"

	"github.com/mengelbart/moqtransport"
)

type errorCode uint64

const (
	errorCodeInvalidNamespace errorCode = iota + 1
	errorCodeInternal
	errorCodeUnknownRoom
	errorCodeDuplicateUsername
	errorCodeUnknownParticipant
)

func namespaceToString(t [][]byte) string {
	res := ""
	for _, t := range t {
		res += string(t)
	}
	return res
}

type sessionManager struct {
	rooms     map[roomID]*room
	roomsLock sync.Mutex

	sessions     []*moqtransport.Transport
	sessionsLock sync.Mutex
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		rooms:        map[roomID]*room{},
		sessions:     []*moqtransport.Transport{},
		sessionsLock: sync.Mutex{},
	}
}

func (m *sessionManager) handle(s *moqtransport.Transport) {
	m.sessionsLock.Lock()
	defer m.sessionsLock.Unlock()
	m.sessions = append(m.sessions, s)
}

func (m *sessionManager) HandleAnnouncement(s *moqtransport.Transport, a *moqtransport.Announcement, arw moqtransport.AnnouncementResponseWriter) {
	parts := strings.SplitN(namespaceToString(a.Namespace()), "/", 4)
	if len(parts) != 4 {
		arw.Reject(uint64(errorCodeInvalidNamespace), "namespace MUST be moq-chat/<room-id>/participant/<username>")
		return
	}
	moqChat, id, participant, username := parts[0], roomID(parts[1]), parts[2], parts[3]
	if moqChat != "moq-chat" {
		arw.Reject(uint64(errorCodeInvalidNamespace), "first part of namespace MUST equal 'moq-chat'")
		return
	}
	if participant != "participant" {
		arw.Reject(uint64(errorCodeInvalidNamespace), "third part of namespace MUST equal 'participant'")
		return
	}
	m.roomsLock.Lock()
	defer m.roomsLock.Unlock()
	room, ok := m.rooms[id]
	if !ok {
		arw.Reject(uint64(errorCodeUnknownRoom), "room not found. to open a room, subscribe to its catalog")
		return
	}
	room.announceUser(username, s, arw)
}

func (m *sessionManager) HandleSubscription(s *moqtransport.Transport, sub moqtransport.Subscription, srw moqtransport.SubscriptionResponseWriter) {
	parts := strings.SplitN(namespaceToString(sub.Namespace), "/", 4)
	if len(parts) != 2 {
		srw.Reject(uint64(errorCodeInvalidNamespace), "invalid namespace")
		return
	}
	m.handleCatalogSubscription(parts, s, sub, srw)
}

func (m *sessionManager) handleCatalogSubscription(namespaceParts []string, s *moqtransport.Transport, sub moqtransport.Subscription, srw moqtransport.SubscriptionResponseWriter) {
	if len(namespaceParts) != 2 {
		panic("invalid namespace parts length")
	}
	moqChat, id := namespaceParts[0], roomID(namespaceParts[1])
	if moqChat != "moq-chat" {
		srw.Reject(uint64(errorCodeInvalidNamespace), "first part of namespace MUST equal 'moq-chat'")
		return
	}
	m.roomsLock.Lock()
	defer m.roomsLock.Unlock()

	room, ok := m.rooms[id]
	if !ok {
		room = newRoom(id)
		m.rooms[id] = room
	}
	room.subscribeCatalog(s, sub, srw)
}
