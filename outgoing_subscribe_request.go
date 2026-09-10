package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type OutgoingSubscribeRequestOption func(*OutgoingSubscribeRequest) error

type OutgoingSubscribeRequest struct {
	logger       *slog.Logger
	requestID    uint64
	session      *Session
	streamWriter messageWriter
	streamReader messageReader
	buffer       chan *Object
	last         *Object
}

func newOutgoingSubscribeRequest(
	requestID uint64,
	session *Session,
	streamWriter messageWriter,
	streamReader messageReader,
	namespace [][]byte,
	trackName []byte,
	parameters ...OutgoingSubscribeRequestOption,
) (*OutgoingSubscribeRequest, error) {
	r := &OutgoingSubscribeRequest{
		logger:       defaultLogger,
		requestID:    requestID,
		session:      session,
		streamWriter: streamWriter,
		streamReader: streamReader,
		buffer:       make(chan *Object, session.subscribeBufferSize),
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
	if err := r.streamWriter.Write(msg); err != nil {
		return nil, err
	}
	r.logger.Debug("sent subscribe request", "requestID", requestID, "namespace", namespace, "trackName", trackName)
	return r, nil
}

// readMessages reads from the request stream until it fails. It must be called
// from a goroutine tracked by the session WaitGroup.
func (r *OutgoingSubscribeRequest) readMessages() {
	for {
		msg, err := r.streamReader.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.session.handleReaderError(err)
			}
			return
		}
		switch msg := msg.(type) {
		case *wire.SubscribeOk:
			if err := r.session.bindTrackAlias(msg.TrackAlias, r); err != nil {
				r.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeDuplicateTrackAlias),
					Reason: err.Error(),
				})
				return
			}
		case *wire.RequestOk:
		case *wire.RequestError:
		default:
			r.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected message type: %T", msg),
			})
			return
		}
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
	// TODO: Send a message to the peer to stop the subscription.
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
	// TODO: Add case for shutdown when request is closed
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case obj := <-r.buffer:
		r.last = obj
		return obj, nil
	}
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
