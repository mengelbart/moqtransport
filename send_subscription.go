package moqtransport

import (
	"sync"
	"time"
)

type SendSubscription struct {
	lock       sync.RWMutex
	responseCh chan error
	closeCh    chan struct{}
	expires    time.Duration

	conn connection

	subscribeID, trackAlias uint64
	namespace, trackname    string
	startGroup, startObject Location
	endGroup, endObject     Location
	parameters              parameters
}

func (s *SendSubscription) Accept() {
	select {
	case <-s.closeCh:
	case s.responseCh <- nil:
	}
}

func (s *SendSubscription) Reject(err error) {
	select {
	case <-s.closeCh:
	case s.responseCh <- err:
	}
}

func (s *SendSubscription) SetExpires(d time.Duration) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.expires = d
}

func (s *SendSubscription) Namespace() string {
	return s.namespace
}

func (s *SendSubscription) Trackname() string {
	return s.trackname
}

func (s *SendSubscription) StartGroup() Location {
	return s.startGroup
}

func (s *SendSubscription) StartObject() Location {
	return s.startObject
}

func (s *SendSubscription) EndGroup() Location {
	return s.endGroup
}

func (s *SendSubscription) EndObject() Location {
	return s.endObject
}

func (s *SendSubscription) NewObjectStream(groupID, objectID, objectSendOrder uint64) (*objectStream, error) {
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newObjectStream(stream, s.subscribeID, s.trackAlias, groupID, objectID, objectSendOrder)
}

func (s *SendSubscription) NewObjectPreferDatagram(groupID, objectID, objectSendOrder uint64, payload []byte) {
	panic("TODO")
}

func (s *SendSubscription) NewTrackHeaderStream(objectSendOrder uint64) (*trackHeaderStream, error) {
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newTrackHeaderStream(stream, s.subscribeID, s.trackAlias, objectSendOrder)
}

func (s *SendSubscription) NewGroupHeaderStream(groupID, objectSendOrder uint64) (*groupHeaderStream, error) {
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newGroupHeaderStream(stream, s.subscribeID, s.trackAlias, groupID, objectSendOrder)
}
