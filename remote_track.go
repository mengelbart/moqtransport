package moqtransport

import (
	"context"
	"errors"
)

type RemoteTrack struct {
	subscribeID uint64
	buffer      chan *Object
}

func newRemoteTrack(id uint64) *RemoteTrack {
	t := &RemoteTrack{
		subscribeID: id,
		buffer:      make(chan *Object),
	}
	return t
}

func (t *RemoteTrack) ReadObject(ctx context.Context) (*Object, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case obj, ok := <-t.buffer:
		if !ok {
			return nil, errors.New("track closed")
		}
		return obj, nil
	}
}

func (t *RemoteTrack) Unsubscribe() {
	panic("TODO")
}

func (t *RemoteTrack) push(o *Object) {
	t.buffer <- o
}
