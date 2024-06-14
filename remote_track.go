package moqtransport

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type RemoteTrack struct {
	logger     *slog.Logger
	responseCh chan subscribeIDer

	session     *Session
	subscribeID uint64
	buffer      chan Object
	closeCh     chan struct{}
}

func newRemoteTrack(id uint64, s *Session) *RemoteTrack {
	t := &RemoteTrack{
		logger:      defaultLogger.WithGroup("MOQ_REMOTE_TRACK"),
		responseCh:  make(chan subscribeIDer),
		session:     s,
		subscribeID: id,
		buffer:      make(chan Object),
		closeCh:     make(chan struct{}),
	}
	return t
}

func (s *RemoteTrack) ReadObject(ctx context.Context) (Object, error) {
	select {
	case <-ctx.Done():
		return Object{}, ctx.Err()
	case obj, ok := <-s.buffer:
		if !ok {
			return Object{}, errors.New("track closed")
		}
		return obj, nil
	}
}

func (s *RemoteTrack) Unsubscribe() {
	s.session.unsubscribe(s.subscribeID)
}

func (s *RemoteTrack) close() {
	close(s.closeCh)
}

func (s *RemoteTrack) push(o Object) {
	s.logger.Info("push object", "object", o)
	select {
	case s.buffer <- o:
	case <-s.closeCh:
	}
}

func (s *RemoteTrack) readTrackHeaderStream(rs ReceiveStream) {
	parser := newParser(quicvarint.NewReader(rs))
	for {
		msg, err := parser.parseStreamHeaderTrackObject()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		s.push(Object{
			GroupID:              msg.GroupID,
			ObjectID:             msg.ObjectID,
			ObjectSendOrder:      0,
			ForwardingPreference: ObjectForwardingPreferenceStreamTrack,
			Payload:              msg.ObjectPayload,
		})
	}
}

func (s *RemoteTrack) readGroupHeaderStream(rs ReceiveStream, groupID uint64) {
	parser := newParser(quicvarint.NewReader(rs))
	for {
		msg, err := parser.parseStreamHeaderGroupObject()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		s.push(Object{
			GroupID:              groupID,
			ObjectID:             msg.ObjectID,
			ObjectSendOrder:      0,
			ForwardingPreference: ObjectForwardingPreferenceStreamGroup,
			Payload:              msg.ObjectPayload,
		})
	}
}
