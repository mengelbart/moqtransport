package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type OutgoingSubscribeRequestOption func(*OutgoingSubscribeRequest) error

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

	// established and responded are only touched from readMessages.
	established bool
	responded   bool

	// requestErr is the first error seen, returned by ReadObject once closed is done.
	errLock    sync.Mutex
	requestErr error
	closed     chan struct{}
	closeOnce  sync.Once
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
		logger:    defaultLogger,
		requestID: requestID,
		session:   session,
		stream:    stream,
		buffer:    make(chan *Object, session.subscribeBufferSize),
		response:  make(chan error, 1),
		closed:    make(chan struct{}),
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
	defer r.markClosed()
	for {
		msg, err := r.stream.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) && !r.isClosed() {
				r.session.handleReaderError(err)
			}
			r.respond(fmt.Errorf("%w: %w", ErrRequestClosed, err))
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

func (r *OutgoingSubscribeRequest) Close() error {
	r.markClosed()
	r.stream.cancel(StreamResetErrorCodeCancelled)
	r.session.removeReceiver(r)
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
		return nil, r.closeError()
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
