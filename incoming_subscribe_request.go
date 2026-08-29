package moqtransport

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type IncomingSubscribeRequest struct {
	logger       *slog.Logger
	session      *Session
	streamWriter controlMessageWriter
	streamReader controlMessageReader

	namespace [][]byte
	name      []byte

	trackAlias uint64
}

func newIncomingSubscribeRequest(msg *wire.Subscribe, session *Session, streamWriter controlMessageWriter, streamReader controlMessageReader) *IncomingSubscribeRequest {
	isr := &IncomingSubscribeRequest{
		logger:       defaultLogger,
		session:      session,
		streamWriter: streamWriter,
		streamReader: streamReader,
		namespace:    msg.TrackNamespace,
		name:         msg.TrackName,
		trackAlias:   0,
	}
	isr.logger.Debug("incoming subscribe request created", "requestID", msg.RequestID, "namespace", msg.TrackNamespace, "trackName", msg.TrackName)
	return isr
}

// readMessages reads from the request stream until it fails. It must be called
// from a goroutine tracked by the session WaitGroup.
func (r *IncomingSubscribeRequest) readMessages() {
	for {
		msg, err := r.streamReader.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.session.handleReaderError(err)
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
	r.trackAlias = trackAlias
	err := r.streamWriter.Write(&wire.SubscribeOk{
		TrackAlias: trackAlias,
	})
	if err != nil {
		// TODO
		panic(err)
	}
}

func (r *IncomingSubscribeRequest) Reject(code SubscribeErrorCode, reason string) {
	err := r.streamWriter.Write(&wire.RequestError{
		ErrorCode:     uint64(code),
		RetryInterval: 0, // TODO: Add retry interval if needed
		ErrorReason:   reason,
	})
	if err != nil {
		// TODO
		panic(err)
	}
}

func (r *IncomingSubscribeRequest) SendDatagram(o Object) error {
	// TODO
	return nil
}

func (r *IncomingSubscribeRequest) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	stream, err := r.session.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	appender := wire.NewAppender(stream, r.session.version)
	return newSubgroup(appender, r.trackAlias, groupID, subgroupID, priority)
}

func (r *IncomingSubscribeRequest) Close() error {
	// TODO
	return nil
}

func (r *IncomingSubscribeRequest) Namespace() [][]byte {
	return r.namespace
}

func (r *IncomingSubscribeRequest) Name() []byte {
	return r.name
}
