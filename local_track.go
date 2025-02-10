package moqtransport

import (
	"context"
	"errors"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	ErrUnsusbcribed     = errors.New("track closed, peer unsubscribed")
	ErrSubscriptionDone = errors.New("track closed, subscription done")
)

type subscribeDoneCallback func(code, count uint64, reason string) error

type localTrack struct {
	conn            Connection
	subscribeID     uint64
	trackAlias      uint64
	subgroupCount   uint64
	fetchStreamLock sync.Mutex
	fetchStream     *FetchStream
	ctx             context.Context
	cancelCtx       context.CancelCauseFunc
	subscribeDone   subscribeDoneCallback
}

func newLocalTrack(conn Connection, subscribeID, trackAlias uint64, onSubscribeDone subscribeDoneCallback) *localTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	publisher := &localTrack{
		conn:            conn,
		subscribeID:     subscribeID,
		trackAlias:      trackAlias,
		subgroupCount:   0,
		fetchStreamLock: sync.Mutex{},
		fetchStream:     nil,
		ctx:             ctx,
		cancelCtx:       cancel,
		subscribeDone:   onSubscribeDone,
	}
	return publisher
}

func (p *localTrack) getFetchStream() (*FetchStream, error) {
	p.fetchStreamLock.Lock()
	defer p.fetchStreamLock.Unlock()

	if err := p.closed(); err != nil {
		return nil, err
	}
	if p.fetchStream != nil {
		return p.fetchStream, nil
	}
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	p.fetchStream, err = newFetchStream(stream, p.subscribeID)
	if err != nil {
		return nil, err
	}
	return p.fetchStream, nil
}

func (p *localTrack) sendDatagram(o Object) error {
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

func (p *localTrack) openSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
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
