package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
)

var errTooManyFetchStreams = errors.New("got too many fetch streams for remote track")

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
	requestID uint64

	logger          *slog.Logger
	unsubscribeFunc func() error
	buffer          chan *Object

	doneCtx       context.Context
	doneCtxCancel context.CancelCauseFunc

	subGroupCount atomic.Uint64
	fetchCount    atomic.Uint64 // should never grow larger than one for now.

	responseChan chan error
}

func newRemoteTrack(requestID uint64, unsubscribeFunc func() error) *RemoteTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	t := &RemoteTrack{
		requestID:       requestID,
		logger:          defaultLogger,
		unsubscribeFunc: unsubscribeFunc,
		buffer:          make(chan *Object, 100),
		doneCtx:         ctx,
		doneCtxCancel:   cancel,
		subGroupCount:   atomic.Uint64{},
		fetchCount:      atomic.Uint64{},
		responseChan:    make(chan error, 1),
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
	if t.unsubscribeFunc != nil {
		return t.unsubscribeFunc()
	}
	return nil
}

// RequestID returns the request ID for this subscription.
func (t *RemoteTrack) RequestID() uint64 {
	return t.requestID
}

func (t *RemoteTrack) readFetchStream(parser objectMessageParser) error {
	if t.fetchCount.Add(1) > 1 {
		return errTooManyFetchStreams
	}
	return t.readStream(parser)
}

func (t *RemoteTrack) readSubgroupStream(parser objectMessageParser) error {
	t.subGroupCount.Add(1)
	return t.readStream(parser)
}

func (t *RemoteTrack) readStream(parser objectMessageParser) error {
	for m, err := range parser.Messages() {
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		t.logger.Debug("subgroup got new object message", "message", m)
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
