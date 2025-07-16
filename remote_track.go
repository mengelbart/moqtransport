package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"
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

	// Expires, groupOrder, ..., parameters are returned in the SUBSCRIBE_OK.
	// They are not updated when sending a SUBSCRIBE_UPDATE message.
	expires         time.Duration
	groupOrder      GroupOrder
	contentExists   bool
	largestLocation *Location // Only set iff ContentExists is true
	parameters      KVPList

	logger          *slog.Logger
	unsubscribeFunc func() error
	updateFunc      func(context.Context, ...SubscribeUpdateOption) error
	buffer          chan *Object

	doneCtx       context.Context
	doneCtxCancel context.CancelCauseFunc

	subGroupCount atomic.Uint64
	fetchCount    atomic.Uint64 // should never grow larger than one for now.

	responseChan chan error
}

// RequestID returns the request ID of the subscription request.
func (t *RemoteTrack) RequestID() uint64 {
	return t.requestID
}

// Expires returns the duration for which the subscription is valid.
// A value of 0 indicates that the subscription does not expire or expires at an unknown time.
func (t *RemoteTrack) Expires() time.Duration {
	return t.expires
}

// GroupOrder returns the group order for this track.
func (t *RemoteTrack) GroupOrder() GroupOrder {
	return t.groupOrder
}

// LargestLocation returns the largest location for this track if content exists.
// Returns false if ContentExists is false or if no LargestLocation was provided.
func (t *RemoteTrack) LargestLocation() (Location, bool) {
	if t.contentExists && t.largestLocation != nil {
		return *t.largestLocation, true
	}
	return Location{}, false
}

// Parameters returns the key-value parameters for this track.
func (t *RemoteTrack) Parameters() KVPList {
	return t.parameters
}

// UpdateSubscription updates the subscription parameters for this track.
// No response is expected according to draft-11 specification.
func (t *RemoteTrack) UpdateSubscription(ctx context.Context, options ...SubscribeUpdateOption) error {
	if t.updateFunc == nil {
		return errors.New("update function not available")
	}
	return t.updateFunc(ctx, options...)
}

func newRemoteTrack(requestID uint64, unsubscribeFunc func() error, updateFunc func(context.Context, ...SubscribeUpdateOption) error) *RemoteTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	t := &RemoteTrack{
		requestID:       requestID,
		logger:          defaultLogger,
		unsubscribeFunc: unsubscribeFunc,
		updateFunc:      updateFunc,
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
