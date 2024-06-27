package moqtransport

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
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

func (t *RemoteTrack) ReadObject(ctx context.Context) (Object, error) {
	select {
	case <-ctx.Done():
		return Object{}, ctx.Err()
	case obj, ok := <-t.buffer:
		if !ok {
			return Object{}, errors.New("track closed")
		}
		return obj, nil
	}
}

func (t *RemoteTrack) Unsubscribe() {
	t.session.unsubscribe(t.subscribeID)
}

func (t *RemoteTrack) close() {
	close(t.closeCh)
}

func (t *RemoteTrack) push(o Object) {
	t.logger.Info("push object", "object", o)
	select {
	case t.buffer <- o:
	case <-t.closeCh:
	}
}

func (t *RemoteTrack) readObjectStream(p *wire.ObjectStreamParser) {
	t.logger.Info("reading object stream")
	defer t.logger.Info("finished reading object stream")
	for {
		msg, err := p.Parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			t.logger.Info("stream canceled by peer", "error", err)
			return
		}
		t.logger.Info("object stream got object", "msg", msg)
		t.push(Object{
			GroupID:              msg.GroupID,
			ObjectID:             msg.ObjectID,
			ObjectSendOrder:      0,
			ForwardingPreference: objectForwardingPreferenceFromMessageType(msg.Type),
			Payload:              msg.ObjectPayload,
		})
	}
}
