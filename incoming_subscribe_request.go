package moqtransport

import (
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire2"
)

type IncomingSubscribeRequest struct {
	logger       *slog.Logger
	version      uint64
	conn         Connection
	streamWriter controlMessageWriter
	streamReader controlMessageReader

	namespace [][]byte
	name      []byte

	trackAlias uint64
}

func newIncomingSubscribeRequest(msg *wire2.Subscribe, version uint64, conn Connection, streamWriter controlMessageWriter, streamReader controlMessageReader) *IncomingSubscribeRequest {
	isr := &IncomingSubscribeRequest{
		logger:       defaultLogger,
		version:      version,
		conn:         conn,
		streamWriter: streamWriter,
		streamReader: streamReader,
		namespace:    msg.TrackNamespace,
		name:         msg.TrackName,
		trackAlias:   0,
	}
	isr.logger.Debug("incoming subscribe request created", "requestID", msg.RequestID, "namespace", msg.TrackNamespace, "trackName", msg.TrackName)
	go isr.readMessages() // TODO: Close request stream
	return isr
}

func (r *IncomingSubscribeRequest) readMessages() {
	for {
		msg, err := r.streamReader.Read()
		if err != nil {
			// TODO
			panic(err)
		}
		switch msg := msg.(type) {
		case *wire2.RequestUpdate:
			// TODO
		default:
			panic(fmt.Sprintf("unexpected message type: %T", msg))
		}
	}
}

func (r *IncomingSubscribeRequest) Accept(trackAlias uint64) {
	r.logger.Debug("accepting subscribe request")
	r.trackAlias = trackAlias
	err := r.streamWriter.Write(&wire2.SubscribeOk{
		TrackAlias: trackAlias,
	})
	if err != nil {
		// TODO
		panic(err)
	}
}

func (r *IncomingSubscribeRequest) Reject(code ErrorCodeSubscribe, reason string) {
	err := r.streamWriter.Write(&wire2.RequestError{
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
	stream, err := r.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	appender := wire2.NewAppender(stream, r.version)
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
