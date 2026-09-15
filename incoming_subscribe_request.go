package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/quic"
)

var (
	errRedirectURIFromClient   = errors.New("only servers may redirect to another connect URI")
	errSubscriptionClosed      = errors.New("subscription is closed")
	errSubscriptionNotAccepted = errors.New("subscription was not accepted")
	errSubgroupsOpen           = errors.New("subscription has open subgroups")
	errSubscriptionCancelled   = errors.New("subscription was cancelled by the subscriber")
	errInvalidObjectStatus     = errors.New("invalid object status")
)

type IncomingSubscribeRequest struct {
	logger  *slog.Logger
	session *Session
	stream  *requestStream
	ctx     context.Context
	cancel  context.CancelCauseFunc

	namespace [][]byte
	name      []byte

	lock        sync.Mutex
	trackAlias  uint64
	accepted    bool
	closed      bool
	streamCount uint64
	subgroups   map[*Subgroup]struct{}
}

func newIncomingSubscribeRequest(msg *wire.Subscribe, session *Session, stream *requestStream) *IncomingSubscribeRequest {
	ctx, cancel := context.WithCancelCause(session.ctx)
	isr := &IncomingSubscribeRequest{
		logger:     defaultLogger,
		session:    session,
		stream:     stream,
		ctx:        ctx,
		cancel:     cancel,
		namespace:  msg.TrackNamespace,
		name:       msg.TrackName,
		trackAlias: 0,
		subgroups:  map[*Subgroup]struct{}{},
	}
	isr.logger.Debug("incoming subscribe request created", "requestID", msg.RequestID, "namespace", msg.TrackNamespace, "trackName", msg.TrackName)
	return isr
}

// readMessages reads from the request stream until it ends. A reset of the
// stream by the subscriber cancels the subscription. It must be called from a
// goroutine tracked by the session WaitGroup.
func (r *IncomingSubscribeRequest) readMessages() {
	for {
		msg, err := r.stream.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.cancelled(err)
			}
			return
		}
		switch msg := msg.(type) {
		case *wire.RequestUpdate:
			// TODO
		default:
			r.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected message type: %T", msg),
			})
			return
		}
	}
}

func (r *IncomingSubscribeRequest) Accept(trackAlias uint64) {
	r.logger.Debug("accepting subscribe request")
	r.lock.Lock()
	r.trackAlias = trackAlias
	r.accepted = true
	r.lock.Unlock()
	err := r.stream.Write(&wire.SubscribeOk{
		TrackAlias: trackAlias,
	})
	if err != nil {
		r.cancelled(err)
	}
}

// Context is cancelled once the subscription ended, either by Close, Reject or
// Redirect or because the subscriber cancelled it. The cause names the reason.
func (r *IncomingSubscribeRequest) Context() context.Context {
	return r.ctx
}

// cancelled ends the subscription after the request stream failed. Open
// subgroups are reset and the subscription state is dropped.
func (r *IncomingSubscribeRequest) cancelled(err error) {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		return
	}
	r.closed = true
	subgroups := make([]*Subgroup, 0, len(r.subgroups))
	for sg := range r.subgroups {
		subgroups = append(subgroups, sg)
	}
	r.lock.Unlock()

	r.logger.Debug("subscription cancelled", "error", err)
	r.stream.cancel(StreamResetErrorCodeCancelled)
	for _, sg := range subgroups {
		sg.stream.Reset(uint32(StreamResetErrorCodeCancelled))
	}
	r.cancel(fmt.Errorf("%w: %w", errSubscriptionCancelled, err))
}

// Reject answers the request with REQUEST_ERROR and asks the peer not to retry.
func (r *IncomingSubscribeRequest) Reject(code RequestErrorCode, reason string) {
	r.sendRequestError(&wire.RequestError{
		ErrorCode:   uint64(code),
		ErrorReason: reason,
	})
}

// RejectRetry answers the request with REQUEST_ERROR and tells the peer it may
// retry after retryAfter.
func (r *IncomingSubscribeRequest) RejectRetry(code RequestErrorCode, reason string, retryAfter time.Duration) {
	r.sendRequestError(&wire.RequestError{
		ErrorCode:     uint64(code),
		RetryInterval: uint64(retryAfter.Milliseconds()) + 1,
		ErrorReason:   reason,
	})
}

// Redirect answers the request with REQUEST_ERROR code REDIRECT pointing to
// another track. An empty uri means the current session, an empty namespace
// together with an empty name means the original track. Only servers may set
// uri, a client passing one gets an error and nothing is sent.
func (r *IncomingSubscribeRequest) Redirect(uri string, namespace [][]byte, name []byte) error {
	if uri != "" && r.session.conn.Perspective() == quic.PerspectiveClient {
		return errRedirectURIFromClient
	}
	r.sendRequestError(&wire.RequestError{
		ErrorCode: uint64(RequestErrorCodeRedirect),
		Redirect: wire.Redirect{
			ConnectURI:     uri,
			TrackNamespace: namespace,
			TrackName:      name,
		},
	})
	return nil
}

func (r *IncomingSubscribeRequest) sendRequestError(msg *wire.RequestError) {
	r.lock.Lock()
	r.closed = true
	r.lock.Unlock()
	defer r.cancel(errSubscriptionClosed)
	if err := r.stream.Write(msg); err != nil {
		r.stream.cancel(StreamResetErrorCodeCancelled)
		return
	}
	if err := r.stream.Close(); err != nil {
		r.stream.cancel(StreamResetErrorCodeCancelled)
	}
}

// SendDatagram sends one object with a payload as a datagram. Errors from the
// connection, e.g. a datagram exceeding the maximum size, are returned to the
// caller so it can fall back to a subgroup.
func (r *IncomingSubscribeRequest) SendDatagram(groupID, objectID uint64, priority uint8, endOfGroup bool, payload []byte) error {
	msg := &wire.DatagramObject{
		GroupID:           groupID,
		ObjectID:          objectID,
		PublisherPriority: priority,
		ObjectPayload:     payload,
	}
	msg.SetEndOfGroup(endOfGroup)
	return r.sendDatagram(msg)
}

// SendDatagramStatus sends a status-only object, e.g. end of group or end of
// track, as a datagram.
func (r *IncomingSubscribeRequest) SendDatagramStatus(groupID, objectID uint64, priority uint8, status ObjectStatus) error {
	if !status.valid() {
		return errInvalidObjectStatus
	}
	msg := &wire.DatagramObject{
		GroupID:           groupID,
		ObjectID:          objectID,
		PublisherPriority: priority,
		ObjectStatus:      uint64(status),
	}
	msg.SetStatus(true)
	return r.sendDatagram(msg)
}

func (r *IncomingSubscribeRequest) sendDatagram(msg *wire.DatagramObject) error {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		return errSubscriptionClosed
	}
	msg.TrackAlias = r.trackAlias
	r.lock.Unlock()

	msg.SetZeroObjectID(msg.ObjectID == 0)
	r.logger.Debug("sending datagram", "trackAlias", msg.TrackAlias, "groupID", msg.GroupID, "objectID", msg.ObjectID, "status", msg.Status())
	return r.session.conn.SendDatagram(msg.AppendDatagram(nil))
}

// OpenSubgroup opens a data stream for a subgroup of the subscription. Every
// subgroup must be closed or reset before the subscription can be closed.
func (r *IncomingSubscribeRequest) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		return nil, errSubscriptionClosed
	}
	trackAlias := r.trackAlias
	r.lock.Unlock()

	stream, err := r.session.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	var subgroup *Subgroup
	subgroup, err = newSubgroup(stream, r.session.version, trackAlias, groupID, subgroupID, priority, func() {
		r.subgroupDone(subgroup)
	})
	r.lock.Lock()
	r.streamCount++
	if err != nil {
		r.lock.Unlock()
		return nil, err
	}
	if r.closed {
		r.lock.Unlock()
		subgroup.Reset(StreamResetErrorCodeCancelled)
		return nil, errSubscriptionClosed
	}
	r.subgroups[subgroup] = struct{}{}
	r.lock.Unlock()
	return subgroup, nil
}

func (r *IncomingSubscribeRequest) subgroupDone(sg *Subgroup) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.subgroups, sg)
}

// Close ends the subscription with PUBLISH_DONE and finishes the request
// stream. It fails while a subgroup of the subscription is still open.
func (r *IncomingSubscribeRequest) Close(status PublishDoneStatusCode, reason string) error {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		return errSubscriptionClosed
	}
	if !r.accepted {
		r.lock.Unlock()
		return errSubscriptionNotAccepted
	}
	if len(r.subgroups) > 0 {
		r.lock.Unlock()
		return errSubgroupsOpen
	}
	r.closed = true
	streamCount := r.streamCount
	r.lock.Unlock()
	defer r.cancel(errSubscriptionClosed)

	r.logger.Debug("closing subscription", "status", status, "streamCount", streamCount)
	if err := r.stream.Write(&wire.PublishDone{
		StatusCode:  uint64(status),
		StreamCount: streamCount,
		ErrorReason: reason,
	}); err != nil {
		r.stream.cancel(StreamResetErrorCodeCancelled)
		return err
	}
	if err := r.stream.Close(); err != nil {
		r.stream.cancel(StreamResetErrorCodeCancelled)
		return err
	}
	return nil
}

func (r *IncomingSubscribeRequest) Namespace() [][]byte {
	return r.namespace
}

func (r *IncomingSubscribeRequest) Name() []byte {
	return r.name
}
