package moqtransport

import (
	"context"
	"fmt"
	"log/slog"
)

type unsubscriber interface {
	unsubscribe(id uint64) error
}

type ErrSubscribeDone struct {
	Status uint64
	Reason string
}

func (e ErrSubscribeDone) Error() string {
	return fmt.Sprintf("subscribe done: status=%v, reason='%v'", e.Status, e.Reason)
}

type RemoteTrack struct {
	logger        *slog.Logger
	subscribeID   uint64
	unsubscriber  unsubscriber
	buffer        chan *Object
	doneCtx       context.Context
	doneCtxCancel context.CancelCauseFunc
}

func newRemoteTrack(id uint64, u unsubscriber) *RemoteTrack {
	ctx, cancel := context.WithCancelCause(context.Background())
	t := &RemoteTrack{
		logger:        defaultLogger,
		subscribeID:   id,
		unsubscriber:  u,
		buffer:        make(chan *Object, 100),
		doneCtx:       ctx,
		doneCtxCancel: cancel,
	}
	return t
}

func (t *RemoteTrack) ReadObject(ctx context.Context) (*Object, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case obj := <-t.buffer:
		return obj, t.doneCtx.Err()
	}
}

func (t *RemoteTrack) Unsubscribe() error {
	return t.unsubscriber.unsubscribe(t.subscribeID)
}

func (t *RemoteTrack) done(status uint64, reason string) {
	t.doneCtxCancel(&ErrSubscribeDone{
		Status: status,
		Reason: reason,
	})
}

func (t *RemoteTrack) push(o *Object) {
	select {
	case t.buffer <- o:
	default:
		t.logger.Info("buffer overflow: dropping incoming object")
	}
}
