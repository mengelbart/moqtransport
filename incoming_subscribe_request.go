package moqtransport

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	errRedirectURIFromClient   = errors.New("only servers may redirect to another connect URI")
	errSubscriptionClosed      = errors.New("subscription is closed")
	errSubscriptionNotAccepted = errors.New("subscription was not accepted")
	errSubgroupsOpen           = errors.New("subscription has open subgroups")
)

type IncomingSubscribeRequest struct {
	logger  *slog.Logger
	session *Session
	stream  *requestStream

	namespace [][]byte
	name      []byte

	lock        sync.Mutex
	trackAlias  uint64
	accepted    bool
	closed      bool
	streamCount uint64
	openStreams int
}

func newIncomingSubscribeRequest(msg *wire.Subscribe, session *Session, stream *requestStream) *IncomingSubscribeRequest {
	isr := &IncomingSubscribeRequest{
		logger:     defaultLogger,
		session:    session,
		stream:     stream,
		namespace:  msg.TrackNamespace,
		name:       msg.TrackName,
		trackAlias: 0,
	}
	isr.logger.Debug("incoming subscribe request created", "requestID", msg.RequestID, "namespace", msg.TrackNamespace, "trackName", msg.TrackName)
	return isr
}

// readMessages reads from the request stream until it fails. It must be called
// from a goroutine tracked by the session WaitGroup.
func (r *IncomingSubscribeRequest) readMessages() {
	for {
		msg, err := r.stream.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.session.closeOnError(err)
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
		r.session.closeOnError(err)
	}
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
	if uri != "" && r.session.conn.Perspective() == PerspectiveClient {
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
	if err := r.stream.Write(msg); err != nil {
		r.session.closeOnError(err)
		return
	}
	if err := r.stream.Close(); err != nil {
		r.session.closeOnError(err)
	}
}

func (r *IncomingSubscribeRequest) SendDatagram(o *Object) error {
	// TODO
	return nil
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
	r.lock.Lock()
	r.streamCount++
	r.openStreams++
	r.lock.Unlock()

	subgroup, err := newSubgroup(stream, r.session.version, trackAlias, groupID, subgroupID, priority, r.subgroupDone)
	if err != nil {
		r.subgroupDone()
		return nil, err
	}
	return subgroup, nil
}

func (r *IncomingSubscribeRequest) subgroupDone() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.openStreams--
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
	if r.openStreams > 0 {
		r.lock.Unlock()
		return errSubgroupsOpen
	}
	r.closed = true
	streamCount := r.streamCount
	r.lock.Unlock()

	r.logger.Debug("closing subscription", "status", status, "streamCount", streamCount)
	if err := r.stream.Write(&wire.PublishDone{
		StatusCode:  uint64(status),
		StreamCount: streamCount,
		ErrorReason: reason,
	}); err != nil {
		r.session.closeOnError(err)
		return err
	}
	if err := r.stream.Close(); err != nil {
		r.session.closeOnError(err)
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
