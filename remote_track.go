package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var errUnknownStreamType = errors.New("received unknown stream type")

var errTooManyFetchStreams = errors.New("got too many fetch streams for remote track")

type unsubscriber interface {
	unsubscribe(id uint64) error
}

// ErrSubscribeDone is returned when reading from a RemoteTrack when the
// subscription has ended.
type ErrSubscribeDone struct {
	Status uint64
	Reason string
}

// Error implements error
func (e ErrSubscribeDone) Error() string {
	return fmt.Sprintf("subscribe done: status=%v, reason='%v'", e.Status, e.Reason)
}

// RemoteTrack is a track provided by the remote peer.
type RemoteTrack struct {
	logger       *slog.Logger
	subscribeID  uint64
	unsubscriber unsubscriber
	buffer       chan *Object

	doneCtx       context.Context
	doneCtxCancel context.CancelCauseFunc

	subGroupCount atomic.Uint64
	fetchCount    atomic.Uint64 // should never grow larger than one for now.
}

func newRemoteTrack(id uint64, u unsubscriber) *RemoteTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	t := &RemoteTrack{
		logger:        defaultLogger,
		subscribeID:   id,
		unsubscriber:  u,
		buffer:        make(chan *Object, 100),
		doneCtx:       ctx,
		doneCtxCancel: cancel,
		subGroupCount: atomic.Uint64{},
		fetchCount:    atomic.Uint64{},
	}
	return t
}

// ReadObject returns the next object received from the peer.
func (t *RemoteTrack) ReadObject(ctx context.Context) (*Object, error) {
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-t.doneCtx.Done():
		return nil, context.Cause(t.doneCtx)
	case obj := <-t.buffer:
		return obj, t.doneCtx.Err()
	}
}

// Close implements io.Closer. Calling close unsubscribes from the subscription.
func (t *RemoteTrack) Close() error {
	return t.unsubscriber.unsubscribe(t.subscribeID)
}

func (t *RemoteTrack) readStream(parser *wire.ObjectStreamParser) error {
	switch parser.Typ {
	case wire.StreamTypeFetch:
		if t.fetchCount.Add(1) > 1 {
			return errTooManyFetchStreams
		}

	case wire.StreamTypeSubgroup:
		t.subGroupCount.Add(1)

	default:
		return errUnknownStreamType
	}
	return t.readSubgroupStream(parser)
}

func (t *RemoteTrack) readSubgroupStream(parser *wire.ObjectStreamParser) error {
	for m, err := range parser.Messages() {
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		t.logger.Info("subgroup got new object message", "message", m)
		payload := make([]byte, len(m.ObjectPayload))
		n := copy(payload, m.ObjectPayload)
		if n != len(m.ObjectPayload) {
			// TODO
			return errors.New("failed to copy object payload: copied less bytes than expected")
		}
		t.push(&Object{
			GroupID:    m.GroupID,
			SubGroupID: m.SubgroupID,
			ObjectID:   m.ObjectID,
			Payload:    payload,
		})
	}
	return nil
}

func (t *RemoteTrack) done(status uint64, reason string) {
	t.doneCtxCancel(&ErrSubscribeDone{
		Status: status,
		Reason: reason,
	})
}

func (t *RemoteTrack) push(o *Object) {
	select {
	case t.buffer <- o:
	default:
		t.logger.Info("buffer overflow: dropping incoming object")
	}
}
