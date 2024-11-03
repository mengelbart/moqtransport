package moqtransport

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go"
)

type localTrackSender struct {
	logger    *slog.Logger
	cancelCtx context.CancelFunc
	cancelWG  sync.WaitGroup
	ctx       context.Context

	conn               Connection
	subscription       *Subscription
	track              LocalTrack
	trackHeaderStream  *trackHeaderStream
	groupHeaderStreams map[uint64]*groupHeaderStream
}

func newLocalTrackSender(conn Connection, s *Subscription, track LocalTrack) *localTrackSender {
	ctx, cancelCtx := context.WithCancel(context.Background())
	sender := &localTrackSender{
		logger:             defaultLogger.WithGroup("MOQ_SEND_SUBSCRIPTION").With("namespace", s.Namespace, "trackname", s.Trackname),
		cancelCtx:          cancelCtx,
		cancelWG:           sync.WaitGroup{},
		ctx:                ctx,
		conn:               conn,
		subscription:       s,
		track:              track,
		trackHeaderStream:  nil,
		groupHeaderStreams: map[uint64]*groupHeaderStream{},
	}
	sender.cancelWG.Add(1)
	go sender.loop()
	return sender
}

func (s *localTrackSender) loop() {
	defer s.cancelWG.Done()
	for o := range s.track.IterUntilClosed(s.ctx) {
		s.sendObject(o)
	}
}

func (s *localTrackSender) sendObject(o Object) {
	s.logger.Info("sending object", "group-id", o.GroupID, "object-id", o.ObjectID)
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

func (s *localTrackSender) sendDatagram(o Object) error {
	om := &wire.ObjectMessage{
		Type:              wire.ObjectDatagramMessageType,
		SubscribeID:       s.subscription.ID,
		TrackAlias:        s.subscription.TrackAlias,
		GroupID:           o.GroupID,
		ObjectID:          o.ObjectID,
		PublisherPriority: o.PublisherPriority,
		ObjectStatus:      0,
		ObjectPayload:     o.Payload,
	}
	buf := make([]byte, 0, 48+len(o.Payload))
	buf = om.Append(buf)
	err := s.conn.SendDatagram(buf)
	if !errors.Is(err, &quic.DatagramTooLargeError{}) {
		return err
	}
	return nil
}

func (s *localTrackSender) sendObjectStream(o Object) error {
	stream, err := s.conn.OpenUniStream()
	if err != nil {
		return err
	}
	os, err := newObjectStream(stream, s.subscription.ID, s.subscription.TrackAlias, o.GroupID, o.ObjectID, o.PublisherPriority)
	if err != nil {
		return err
	}
	if _, err := os.Write(o.Payload); err != nil {
		return err
	}
	return os.Close()
}

func (s *localTrackSender) sendTrackHeaderStream(o Object) error {
	if s.trackHeaderStream == nil {
		stream, err := s.conn.OpenUniStream()
		if err != nil {
			return err
		}
		ts, err := newTrackHeaderStream(stream, s.subscription.ID, s.subscription.TrackAlias, o.PublisherPriority)
		if err != nil {
			return err
		}
		s.trackHeaderStream = ts
	}
	_, err := s.trackHeaderStream.writeObject(o.GroupID, o.ObjectID, o.Payload)
	return err
}

func (s *localTrackSender) sendGroupHeaderStream(o Object) error {
	gs, ok := s.groupHeaderStreams[o.GroupID]
	if !ok {
		var stream SendStream
		var err error
		stream, err = s.conn.OpenUniStream()
		if err != nil {
			return err
		}
		gs, err = newGroupHeaderStream(stream, s.subscription.ID, s.subscription.TrackAlias, o.GroupID, o.PublisherPriority)
		if err != nil {
			return err
		}
		s.groupHeaderStreams[o.GroupID] = gs
	}
	_, err := gs.writeObject(o.ObjectID, o.Payload)
	return err
}

func (s *localTrackSender) Close() error {
	s.cancelCtx()
	s.cancelWG.Wait()
	return nil
}
