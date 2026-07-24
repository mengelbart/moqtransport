package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire2"
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
	msg := &wire2.Subscribe{
		RequestID:      requestID,
		TrackNamespace: namespace,
		TrackName:      trackName,
		Parameters:     nil, // TODO: Add parameters if needed
	}
	if err := r.streamWriter.Write(msg); err != nil {
		return nil, err
	}
	go r.readMessages() // TODO: Close request stream
	return r, nil
}

func (r *OutgoingSubscribeRequest) readMessages() {
	for {
		msg, err := r.streamReader.Read()
		if err != nil {
			// TODO
			panic(err)
		}
		switch msg := msg.(type) {
		case *wire2.SubscribeOk:
			// TODO
		case *wire2.RequestOk:
		case *wire2.RequestError:
		default:
			panic(fmt.Sprintf("unexpected message type: %T", msg))
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

func (r *OutgoingSubscribeRequest) readStream(parser objectMessageParser) {
	for m, err := range parser.Messages() {
		if err != nil {
			// TODO
			panic(err)
		}
		payload := make([]byte, len(m.ObjectPayload))
		n := copy(payload, m.ObjectPayload)
		if n != len(m.ObjectPayload) {
			// TODO
			panic(errors.New("failed to copy object payload: copied less bytes than expected"))
		}
		r.push(&Object{
			GroupID:    m.GroupID,
			SubGroupID: m.SubgroupID,
			ObjectID:   m.ObjectID,
			Payload:    payload,
		})
	}
}

func (r *OutgoingSubscribeRequest) Close() error {
	// TODO: Implement close logic
	return nil
}

func (r *OutgoingSubscribeRequest) ReadObject(ctx context.Context) (*Object, error) {
	// TODO: Add case for shutdown when request is closed
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case obj := <-r.buffer:
		return obj, nil
	}
}
