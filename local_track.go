package moqtransport

import (
	"context"
	"errors"
	"sync"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
)

var (
	ErrUnsusbcribed     = errors.New("track closed, peer unsubscribed")
	ErrSubscriptionDone = errors.New("track closed, subscription done")
)

type subscribeDoneCallback func(code, count uint64, reason string) error

type localTrack struct {
	qlogger *qlog.Logger

	conn            Connection
	requestID       uint64
	trackAlias      uint64
	subgroupCount   uint64
	fetchStreamLock sync.Mutex
	fetchStream     *FetchStream
	ctx             context.Context
	cancelCtx       context.CancelCauseFunc
	subscribeDone   subscribeDoneCallback
}

func newLocalTrack(conn Connection, requestID, trackAlias uint64, onSubscribeDone subscribeDoneCallback, qlogger *qlog.Logger) *localTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	lt := &localTrack{
		qlogger:         qlogger,
		conn:            conn,
		requestID:       requestID,
		trackAlias:      trackAlias,
		subgroupCount:   0,
		fetchStreamLock: sync.Mutex{},
		fetchStream:     nil,
		ctx:             ctx,
		cancelCtx:       cancel,
		subscribeDone:   onSubscribeDone,
	}
	return lt
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
	p.fetchStream, err = newFetchStream(stream, p.requestID, p.qlogger)
	if err != nil {
		return nil, err
	}
	return p.fetchStream, nil
}

func (p *localTrack) sendDatagram(o Object) error {
	if err := p.closed(); err != nil {
		return err
	}
	om := &wire.ObjectDatagramMessage{
		TrackAlias:             p.trackAlias,
		GroupID:                o.GroupID,
		ObjectID:               o.ObjectID,
		PublisherPriority:      0,
		ObjectExtensionHeaders: nil,
		ObjectStatus:           0,
		ObjectPayload:          o.Payload,
	}
	var buf []byte
	buf = om.AppendDatagram(buf)
	if p.qlogger != nil {
		eth := slices.Collect(slices.Map(
			om.ObjectExtensionHeaders,
			func(e wire.KeyValuePair) moqt.ExtensionHeader {
				return moqt.ExtensionHeader{
					HeaderType:   0, // TODO
					HeaderValue:  0, // TODO
					HeaderLength: 0, // TODO
					Payload:      qlog.RawInfo{},
				}
			}),
		)
		name := moqt.ObjectDatagramEventCreated
		if len(om.ObjectPayload) > 0 {
			name = moqt.ObjectDatagramStatusEventCreated
		}
		p.qlogger.Log(moqt.ObjectDatagramEvent{
			EventName:              name,
			TrackAlias:             om.TrackAlias,
			GroupID:                om.GroupID,
			ObjectID:               om.ObjectID,
			PublisherPriority:      om.PublisherPriority,
			ExtensionHeadersLength: uint64(len(om.ObjectExtensionHeaders)),
			ExtensionHeaders:       eth,
			ObjectStatus:           uint64(om.ObjectStatus),
			Payload: qlog.RawInfo{
				Length:        uint64(len(om.ObjectPayload)),
				PayloadLength: uint64(len(om.ObjectPayload)),
				Data:          om.ObjectPayload,
			},
		})
	}
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
	return newSubgroup(stream, p.trackAlias, groupID, subgroupID, priority, p.qlogger)
}

func (s *localTrack) close(code uint64, reason string) error {
	s.cancelCtx(ErrSubscriptionDone)
	if s.subscribeDone != nil {
		return s.subscribeDone(code, s.subgroupCount, reason)
	}
	return nil
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
