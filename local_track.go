package moqtransport

import (
	"context"
	"errors"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	ErrUnsusbcribed     = errors.New("track closed, peer unsubscribed")
	ErrSubscriptionDone = errors.New("track closed, subscription done")
)

type subscribeDoneCallback func(code, count uint64, reason string) error

type localTrack struct {
	conn          Connection
	subscribeID   uint64
	trackAlias    uint64
	subgroupCount uint64
	fetchStream   SendStream
	ctx           context.Context
	cancelCtx     context.CancelCauseFunc
	subscribeDone subscribeDoneCallback
}

func newLocalTrack(conn Connection, subscribeID, trackAlias uint64, onSubscribeDone subscribeDoneCallback) *localTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	publisher := &localTrack{
		conn:          conn,
		subscribeID:   subscribeID,
		trackAlias:    trackAlias,
		subgroupCount: 0,
		fetchStream:   nil,
		ctx:           ctx,
		cancelCtx:     cancel,
		subscribeDone: onSubscribeDone,
	}
	return publisher
}

func (p *localTrack) OpenFetchStream() (*FetchStream, error) {
	if err := p.closed(); err != nil {
		return nil, err
	}
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	fs, err := newFetchStream(stream, p.subscribeID)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func (p *localTrack) SendDatagram(o Object) error {
	if err := p.closed(); err != nil {
		return err
	}
	om := &wire.ObjectMessage{
		TrackAlias:        0,
		GroupID:           o.GroupID,
		SubgroupID:        o.SubGroupID,
		ObjectID:          o.ObjectID,
		PublisherPriority: 0,
		ObjectStatus:      0,
		ObjectPayload:     o.Payload,
	}
	buf := make([]byte, 0, 48+len(o.Payload))
	buf = om.AppendDatagram(buf)
	return p.conn.SendDatagram(buf)
}

func (p *localTrack) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	if err := p.closed(); err != nil {
		return nil, err
	}
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	p.subgroupCount++
	return newSubgroup(stream, p.trackAlias, groupID, subgroupID, priority)
}

func (s *localTrack) close(code uint64, reason string) error {
	s.cancelCtx(ErrSubscriptionDone)
	return s.subscribeDone(code, s.subgroupCount, reason)
}

func (s *localTrack) unsubscribe() {
	s.cancelCtx(ErrUnsusbcribed)
}

func (s *localTrack) closed() error {
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	default:
		return nil
	}
}
