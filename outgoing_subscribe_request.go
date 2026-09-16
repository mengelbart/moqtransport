package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/quic"
)

type OutgoingSubscribeRequestOption func(*OutgoingSubscribeRequest) error

// WithGoAwayHandler registers f to be called when the publisher sends GOAWAY
// on the request stream, asking the subscriber to re-issue the request on the
// session at uri, or the current session if uri is empty. timeout is how long
// the publisher intends to keep the request open, zero means no specific
// timeout. f is called from the goroutine reading the request stream.
func WithGoAwayHandler(f func(uri string, timeout time.Duration)) OutgoingSubscribeRequestOption {
	return func(r *OutgoingSubscribeRequest) error {
		r.goAwayHandler = f
		return nil
	}
}

type OutgoingSubscribeRequest struct {
	logger    *slog.Logger
	requestID uint64
	session   *Session
	stream    *requestStream
	buffer    chan *Object
	last      *Object

	// response receives the outcome of the request: nil after SUBSCRIBE_OK,
	// otherwise the error that ended the request.
	response chan error

	goAwayHandler func(uri string, timeout time.Duration)
	goAwaySent    atomic.Bool

	// established, responded and goAwayReceived are only touched from
	// readMessages.
	established    bool
	responded      bool
	goAwayReceived bool

	// requestErr is the first error seen, returned by ReadObject once closed is done.
	errLock    sync.Mutex
	requestErr error
	closed     chan struct{}
	closeOnce  sync.Once

	// streamsLock guards the data stream accounting. done is the PUBLISH_DONE
	// received for the subscription, streamsDone is closed once the number of
	// streams it announced arrived and all of them ended.
	streamsLock     sync.Mutex
	done            *PublishDone
	streamsReceived uint64
	openStreams     map[*subgroupStream]struct{}
	streamsDone     chan struct{}
	dropped         bool
}

func newOutgoingSubscribeRequest(
	requestID uint64,
	session *Session,
	stream *requestStream,
	namespace [][]byte,
	trackName []byte,
	parameters ...OutgoingSubscribeRequestOption,
) (*OutgoingSubscribeRequest, error) {
	r := &OutgoingSubscribeRequest{
		logger:      defaultLogger,
		requestID:   requestID,
		session:     session,
		stream:      stream,
		buffer:      make(chan *Object, session.subscribeBufferSize),
		response:    make(chan error, 1),
		closed:      make(chan struct{}),
		openStreams: make(map[*subgroupStream]struct{}),
		streamsDone: make(chan struct{}),
	}
	for _, opt := range parameters {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	msg := &wire.Subscribe{
		RequestID:      requestID,
		TrackNamespace: namespace,
		TrackName:      trackName,
		Parameters:     nil, // TODO: Add parameters if needed
	}
	if err := r.stream.Write(msg); err != nil {
		return nil, err
	}
	r.logger.Debug("sent subscribe request", "requestID", requestID, "namespace", namespace, "trackName", trackName)
	return r, nil
}

// readMessages reads from the request stream until it fails. It must be called
// from a goroutine tracked by the session WaitGroup.
func (r *OutgoingSubscribeRequest) readMessages() {
	publishDone := false
	defer func() {
		if !publishDone {
			r.markClosed()
		}
	}()
	for {
		msg, err := r.stream.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) && !r.isClosed() {
				r.session.closeOnError(err)
			}
			if !publishDone {
				r.respond(fmt.Errorf("%w: %w", ErrRequestClosed, err))
			}
			return
		}
		if publishDone {
			r.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("%T after PUBLISH_DONE", msg),
			})
			return
		}
		switch msg := msg.(type) {
		case *wire.SubscribeOk:
			if r.established {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "duplicate SUBSCRIBE_OK",
				})
				return
			}
			if err := r.session.bindTrackAlias(msg.TrackAlias, r); err != nil {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeDuplicateTrackAlias),
					Reason: err.Error(),
				})
				return
			}
			r.established = true
			r.respond(nil)
		case *wire.RequestOk:
			if !r.established {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "REQUEST_OK before SUBSCRIBE_OK",
				})
				return
			}
		case *wire.RequestError:
			if !r.established {
				reqErr, sessErr := r.session.requestErrorFromWire(msg, false)
				if sessErr != nil {
					r.session.closeWithError(sessErr)
					return
				}
				r.respond(reqErr)
				return
			}
		case *wire.PublishDone:
			if !r.established {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "PUBLISH_DONE before SUBSCRIBE_OK",
				})
				return
			}
			publishDone = true
			r.setPublishDone(&PublishDone{
				StatusCode:  PublishDoneStatusCode(msg.StatusCode),
				Reason:      msg.ErrorReason,
				StreamCount: msg.StreamCount,
			})
			if err := r.session.goTracked(r.awaitTeardown); err != nil {
				return
			}
		case *wire.GoAwayReq:
			if r.goAwayReceived {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "duplicate GOAWAY on request stream",
				})
				return
			}
			r.goAwayReceived = true
			if err := r.session.validateGoAwayURI(msg.NewSessionURI); err != nil {
				r.session.closeWithError(err)
				return
			}
			if r.goAwayHandler != nil {
				r.goAwayHandler(msg.NewSessionURI, time.Duration(msg.Timeout)*time.Millisecond)
			}
		default:
			r.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected message type: %T", msg),
			})
			return
		}
	}
}

// respond delivers the request outcome to Subscribe on the first call and
// records the first error for ReadObject. Only called from readMessages.
func (r *OutgoingSubscribeRequest) respond(err error) {
	r.errLock.Lock()
	if r.requestErr == nil {
		r.requestErr = err
	}
	r.errLock.Unlock()
	if !r.responded {
		r.responded = true
		r.response <- err
	}
}

func (r *OutgoingSubscribeRequest) markClosed() {
	r.closeOnce.Do(func() { close(r.closed) })
}

func (r *OutgoingSubscribeRequest) addSubgroupStream(s *subgroupStream) {
	r.streamsLock.Lock()
	defer r.streamsLock.Unlock()
	if r.dropped {
		s.stop()
		return
	}
	r.streamsReceived++
	r.openStreams[s] = struct{}{}
	r.accountStreams()
}

func (r *OutgoingSubscribeRequest) removeSubgroupStream(s *subgroupStream) {
	r.streamsLock.Lock()
	defer r.streamsLock.Unlock()
	delete(r.openStreams, s)
	r.accountStreams()
}

func (r *OutgoingSubscribeRequest) setPublishDone(done *PublishDone) {
	r.streamsLock.Lock()
	defer r.streamsLock.Unlock()
	r.done = done
	r.accountStreams()
}

// accountStreams compares the data streams seen against the Stream Count of
// PUBLISH_DONE and closes streamsDone once they all arrived and ended, so the
// teardown does not have to wait for the timeout. More streams than announced
// fail the session. Before PUBLISH_DONE there is nothing to compare against.
// It must be called with streamsLock held.
func (r *OutgoingSubscribeRequest) accountStreams() {
	if r.done == nil {
		return
	}
	if r.streamsReceived > r.done.StreamCount {
		r.session.closeWithError(&SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "more data streams than announced in PUBLISH_DONE",
		})
		return
	}
	if r.streamsReceived == r.done.StreamCount && len(r.openStreams) == 0 {
		select {
		case <-r.streamsDone:
		default:
			close(r.streamsDone)
		}
	}
}

// dropState removes the subscription from the session and stops the data
// streams still open for it.
func (r *OutgoingSubscribeRequest) dropState() {
	r.session.removeReceiver(r)
	r.streamsLock.Lock()
	defer r.streamsLock.Unlock()
	r.dropped = true
	for s := range r.openStreams {
		s.stop()
	}
}

// awaitTeardown drops the subscription state once every data stream announced
// in PUBLISH_DONE has ended or the timeout expired. It must be called from a
// goroutine tracked by the session WaitGroup.
func (r *OutgoingSubscribeRequest) awaitTeardown() {
	timer := time.NewTimer(r.session.publishDoneTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-r.streamsDone:
	case <-r.session.ctx.Done():
	case <-r.closed:
		return
	}
	r.dropState()
	r.streamsLock.Lock()
	done := r.done
	r.streamsLock.Unlock()
	r.errLock.Lock()
	if r.requestErr == nil {
		r.requestErr = done
	}
	r.errLock.Unlock()
	r.markClosed()
}

func (r *OutgoingSubscribeRequest) isClosed() bool {
	select {
	case <-r.closed:
		return true
	default:
		return false
	}
}

func (t *OutgoingSubscribeRequest) push(o *Object) {
	if o.ForwardingPreference == ObjectForwardingPreferenceDatagram {
		select {
		case t.buffer <- o:
		default:
			t.logger.Info("buffer overflow: dropping incoming object")
		}
		return
	}
	// An object read from a data stream holds that stream, so it waits here
	// rather than being dropped.
	select {
	case t.buffer <- o:
	case <-t.session.ctx.Done():
	}
}

// GoAway sends GOAWAY on the request stream, telling the publisher that the
// request is being migrated to the session at uri, or to the current session
// if uri is empty. Only servers may pass a non-empty uri. timeout is announced
// to the peer as the time the request stays open, zero means no specific
// timeout. No timer runs, the caller is expected to end the request itself
// with Close once it migrated.
func (r *OutgoingSubscribeRequest) GoAway(uri string, timeout time.Duration) error {
	if uri != "" && r.session.conn.Perspective() == quic.PerspectiveClient {
		return ErrGoAwayURIFromClient
	}
	if r.isClosed() {
		return r.closeError()
	}
	if !r.goAwaySent.CompareAndSwap(false, true) {
		return ErrGoAwaySent
	}
	r.logger.Debug("sending GOAWAY on request stream", "uri", uri, "timeout", timeout)
	return r.stream.Write(&wire.GoAwayReq{
		NewSessionURI: uri,
		Timeout:       uint64(timeout.Milliseconds()),
	})
}

func (r *OutgoingSubscribeRequest) Close() error {
	r.markClosed()
	r.stream.cancel(StreamResetErrorCodeCancelled)
	r.dropState()
	return r.releaseLast()
}

// ReadObject returns the next object of the subscription. The payload of the
// object returned by the previous call is released, so it must be read before
// the next call. ReadObject must be called from one goroutine at a time.
func (r *OutgoingSubscribeRequest) ReadObject(ctx context.Context) (*Object, error) {
	r.logger.Debug("waiting for next object")
	if err := r.releaseLast(); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case obj := <-r.buffer:
		r.last = obj
		return obj, nil
	case <-r.closed:
		select {
		case obj := <-r.buffer:
			r.last = obj
			return obj, nil
		default:
			return nil, r.closeError()
		}
	}
}

func (r *OutgoingSubscribeRequest) closeError() error {
	r.errLock.Lock()
	defer r.errLock.Unlock()
	if r.requestErr != nil {
		return r.requestErr
	}
	return ErrRequestClosed
}

func (r *OutgoingSubscribeRequest) releaseLast() error {
	if r.last == nil {
		return nil
	}
	if err := r.last.Close(); err != nil {
		return err
	}
	r.last = nil
	return nil
}
