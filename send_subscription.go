package moqtransport

import (
	"errors"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

var errUnsubscribed = errors.New("peer unsubscribed")

type SendSubscription struct {
	lock    sync.RWMutex
	closeCh chan struct{}
	expires time.Duration

	conn Connection

	subscribeID, trackAlias uint64
	namespace, trackname    string
	startGroup, startObject Location
	endGroup, endObject     Location
	parameters              parameters
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

func (s *SendSubscription) unsubscribe() error {
	close(s.closeCh)
	return nil
}

func (s *SendSubscription) NewObjectStream(groupID, objectID, objectSendOrder uint64) (*objectStream, error) {
	select {
	case <-s.closeCh:
		return nil, errUnsubscribed
	default:
	}
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newObjectStream(stream, s.subscribeID, s.trackAlias, groupID, objectID, objectSendOrder)
}

func (s *SendSubscription) NewObjectPreferDatagram(groupID, objectID, objectSendOrder uint64, payload []byte) error {
	select {
	case <-s.closeCh:
		return errUnsubscribed
	default:
	}
	o := objectMessage{
		datagram:        true,
		SubscribeID:     s.subscribeID,
		TrackAlias:      s.trackAlias,
		GroupID:         groupID,
		ObjectID:        objectID,
		ObjectSendOrder: objectSendOrder,
		ObjectPayload:   payload,
	}
	buf := make([]byte, 0, 48+len(o.ObjectPayload))
	buf = o.append(buf)
	err := s.conn.SendDatagram(buf)
	if err == nil {
		return nil
	}
	if !errors.Is(err, &quic.DatagramTooLargeError{}) {
		return err
	}
	os, err := s.NewObjectStream(groupID, objectID, objectSendOrder)
	if err != nil {
		return err
	}
	_, err = os.Write(buf)
	if err != nil {
		return err
	}
	return os.Close()
}

func (s *SendSubscription) NewTrackHeaderStream(objectSendOrder uint64) (*TrackHeaderStream, error) {
	select {
	case <-s.closeCh:
		return nil, errUnsubscribed
	default:
	}
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newTrackHeaderStream(stream, s.subscribeID, s.trackAlias, objectSendOrder)
}

func (s *SendSubscription) NewGroupHeaderStream(groupID, objectSendOrder uint64) (*groupHeaderStream, error) {
	select {
	case <-s.closeCh:
		return nil, errUnsubscribed
	default:
	}
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newGroupHeaderStream(stream, s.subscribeID, s.trackAlias, groupID, objectSendOrder)
}
