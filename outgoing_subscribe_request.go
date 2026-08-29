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
	streamWriter controlMessageWriter
	streamReader controlMessageReader
	buffer       chan *Object
}

func newOutgoingSubscribeRequest(
	requestID uint64,
	session *Session,
	streamWriter controlMessageWriter,
	streamReader controlMessageReader,
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
		buffer:       make(chan *Object, 100), // TODO: Make buffer size configurable
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
			r.session.setTrackAliasForRequest(r.requestID, msg.TrackAlias)
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
	select {
	case t.buffer <- o:
	default:
		t.logger.Info("buffer overflow: dropping incoming object")
	}
}

// readStream reads objects from a subgroup stream until it ends. It must be
// called from a goroutine tracked by the session WaitGroup.
func (r *OutgoingSubscribeRequest) readStream(header *wire.SubgroupHeader, parser controlMessageReader) {
	for {
		m, err := parser.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.session.handleReaderError(err)
			}
			return
		}
		o, ok := m.(*wire.ObjectStream)
		if !ok {
			r.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected message type: %T", m),
			})
			return
		}
		payload := make([]byte, len(o.ObjectPayload))
		copy(payload, o.ObjectPayload)
		r.logger.Debug("received object", "groupID", header.GroupID, "subgroupID", header.SubgroupID, "payloadLength", len(payload))
		r.push(&Object{
			// TODO: Set GroupID and ObjectID from header and o
			Payload: payload,
		})
	}
}

func (r *OutgoingSubscribeRequest) Close() error {
	// TODO: Implement close logic
	return nil
}

func (r *OutgoingSubscribeRequest) ReadObject(ctx context.Context) (*Object, error) {
	r.logger.Debug("waiting for next object")
	// TODO: Add case for shutdown when request is closed
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case obj := <-r.buffer:
		return obj, nil
	}
}
