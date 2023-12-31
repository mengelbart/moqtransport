package moqtransport

import (
	"sync"
	"time"
)

type Subscription struct {
	lock       sync.RWMutex
	track      *SendTrack
	responseCh chan error
	closeCh    chan struct{}
	expires    time.Duration

	namespace, trackname    string
	startGroup, startObject Location
	endGroup, endObject     Location
	parameters              parameters
}

func (s *Subscription) Accept() *SendTrack {
	select {
	case <-s.closeCh:
	case s.responseCh <- nil:
	}
	return s.track
}

func (s *Subscription) Reject(err error) {
	select {
	case <-s.closeCh:
	case s.responseCh <- err:
	}
}

func (s *Subscription) SetTrackID(id uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.track.id = id
}

func (s *Subscription) SetExpires(d time.Duration) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.expires = d
}

func (s *Subscription) Namespace() string {
	return s.namespace
}

func (s *Subscription) Trackname() string {
	return s.trackname
}

func (s *Subscription) TrackID() uint64 {
	return s.track.id
}

func (s *Subscription) StartGroup() Location {
	return s.startGroup
}

func (s *Subscription) StartObject() Location {
	return s.startObject
}

func (s *Subscription) EndGroup() Location {
	return s.endGroup
}

func (s *Subscription) EndObject() Location {
	return s.endObject
}
