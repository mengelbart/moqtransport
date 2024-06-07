package moqtransport

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/quic-go/quic-go"
)

var errUnsubscribed = errors.New("peer unsubscribed")

// TODO: Unexport sendSubscription type
type sendSubscription struct {
	logger    *slog.Logger
	cancelCtx context.CancelFunc
	cancelWG  sync.WaitGroup
	ctx       context.Context

	subscriptionIDinTrack   subscriberID
	subscribeID, trackAlias uint64
	namespace, trackname    string
	conn                    Connection
	objectCh                chan Object
	trackHeaderStream       *trackHeaderStream
	groupHeaderStreams      map[uint64]*groupHeaderStream
}

func newSendSubscription(conn Connection, subscribeID, trackAlias uint64, namespace, trackname string) *sendSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	sub := &sendSubscription{
		logger:                defaultLogger.WithGroup("MOQ_SEND_SUBSCRIPTION"),
		cancelCtx:             cancelCtx,
		cancelWG:              sync.WaitGroup{},
		ctx:                   ctx,
		subscriptionIDinTrack: -1,
		subscribeID:           subscribeID,
		trackAlias:            trackAlias,
		namespace:             namespace,
		trackname:             trackname,
		conn:                  conn,
		objectCh:              make(chan Object, 64),
		trackHeaderStream:     nil,
		groupHeaderStreams:    map[uint64]*groupHeaderStream{},
	}
	sub.cancelWG.Add(1)
	go sub.loop()
	return sub
}

func (s *sendSubscription) loop() {
	defer s.cancelWG.Done()
	for {
		select {
		case o := <-s.objectCh:
			s.sendObject(o)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *sendSubscription) sendObject(o Object) {
	s.logger.Info("sending object", "object", o)
	switch o.ForwardingPreference {
	case ObjectForwardingPreferenceDatagram:
		if err := s.sendDatagram(o); err != nil {
			panic(err)
		}
	case ObjectForwardingPreferenceStream:
		if err := s.sendObjectStream(o); err != nil {
			panic(err)
		}
	case ObjectForwardingPreferenceStreamGroup:
		if err := s.sendGroupHeaderStream(o); err != nil {
			panic(err)
		}
	case ObjectForwardingPreferenceStreamTrack:
		if err := s.sendTrackHeaderStream(o); err != nil {
			panic(err)
		}
	}
}

func (s *sendSubscription) WriteObject(o Object) error {
	select {
	case s.objectCh <- o:
	case <-s.ctx.Done():
		return errUnsubscribed
	default:
		panic("TODO: improve queuing/caching for slow subscribers?")
	}
	return nil
}

func (s *sendSubscription) sendDatagram(o Object) error {
	om := objectMessage{
		datagram:        true,
		SubscribeID:     s.subscribeID,
		TrackAlias:      s.trackAlias,
		GroupID:         o.GroupID,
		ObjectID:        o.ObjectID,
		ObjectSendOrder: o.ObjectSendOrder,
		ObjectPayload:   o.Payload,
	}
	buf := make([]byte, 0, 48+len(o.Payload))
	buf = om.append(buf)
	err := s.conn.SendDatagram(buf)
	if !errors.Is(err, &quic.DatagramTooLargeError{}) {
		return err
	}
	return nil
}

func (s *sendSubscription) sendObjectStream(o Object) error {
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return err
	}
	os, err := newObjectStream(stream, s.subscribeID, s.trackAlias, o.GroupID, o.ObjectID, o.ObjectSendOrder)
	if err != nil {
		return err
	}
	if _, err := os.Write(o.Payload); err != nil {
		return err
	}
	return os.Close()
}

func (s *sendSubscription) sendTrackHeaderStream(o Object) error {
	if s.trackHeaderStream == nil {
		stream, err := s.conn.OpenUniStream()
		if err != nil {
			return err
		}
		ts, err := newTrackHeaderStream(stream, s.subscribeID, s.trackAlias, o.ObjectSendOrder)
		if err != nil {
			return err
		}
		s.trackHeaderStream = ts
	}
	_, err := s.trackHeaderStream.writeObject(o.GroupID, o.ObjectID, o.Payload)
	return err
}

func (s *sendSubscription) sendGroupHeaderStream(o Object) error {
	gs, ok := s.groupHeaderStreams[o.GroupID]
	if !ok {
		var stream SendStream
		var err error
		stream, err = s.conn.OpenUniStream()
		if err != nil {
			return err
		}
		gs, err = newGroupHeaderStream(stream, s.subscribeID, s.trackAlias, o.GroupID, o.ObjectSendOrder)
		if err != nil {
			return err
		}
		s.groupHeaderStreams[o.GroupID] = gs
	}
	_, err := gs.writeObject(o.ObjectID, o.Payload)
	return err
}

func (s *sendSubscription) Close() error {
	s.cancelCtx()
	s.cancelWG.Wait()
	return nil
}
