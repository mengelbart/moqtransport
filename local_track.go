package moqtransport

import (
	"context"
	"errors"
	"io"
	"sync"
)

type subscriberMapKey struct {
	trackID, subscribeID uint64
}

type subscriberAction bool

const (
	subscriberActionAdd    subscriberAction = true
	subscriberActionRemove subscriberAction = false
)

type subscriberOp struct {
	action                   subscriberAction
	trackAlias, subscriberID uint64
	subscriber               ObjectWriter
	errCh                    chan error
}

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceDatagram ObjectForwardingPreference = iota
	ObjectForwardingPreferenceStream
	ObjectForwardingPreferenceStreamGroup
	ObjectForwardingPreferenceStreamTrack
)

type Object struct {
	GroupID              uint64
	ObjectID             uint64
	ObjectSendOrder      uint64
	ForwardingPreference ObjectForwardingPreference

	Payload []byte
}

// An ObjectWriter allows sending objects using.
type ObjectWriter interface {
	WriteObject(Object) error
	io.Closer
}

// A LocalTrack is a local media source. Writing objects to the track will relay
// the objects to all subscribers.
// All methods are safe for concurrent use. Ordering of objects is only
// guaranteed within MultiObjectStreams. LocalTracks must be created with
// NewLocalTrack to ensure proper initialization.
type LocalTrack struct {
	ID        uint64
	Namespace string
	Name      string

	cancelCtx context.CancelFunc
	cancelWG  sync.WaitGroup
	ctx       context.Context

	subscriberCh      chan subscriberOp
	subscribers       map[subscriberMapKey]ObjectWriter
	objectCh          chan Object
	subscriberCountCh chan int
}

// NewLocalTrack creates a new LocalTrack
func NewLocalTrack(id uint64, namespace, trackname string) *LocalTrack {
	ctx, cancelCtx := context.WithCancel(context.Background())
	lt := &LocalTrack{
		ID:                id,
		Namespace:         namespace,
		Name:              trackname,
		cancelCtx:         cancelCtx,
		cancelWG:          sync.WaitGroup{},
		ctx:               ctx,
		subscriberCh:      make(chan subscriberOp),
		subscribers:       map[subscriberMapKey]ObjectWriter{},
		objectCh:          make(chan Object),
		subscriberCountCh: make(chan int),
	}
	lt.cancelWG.Add(1)
	go lt.loop()
	return lt
}

func (t *LocalTrack) manageSubscriber(op subscriberOp) error {
	key := subscriberMapKey{
		trackID:     op.trackAlias,
		subscribeID: op.subscriberID,
	}
	_, ok := t.subscribers[key]
	switch op.action {
	case subscriberActionAdd:
		if ok {
			return errors.New("duplicate subscriber ID in session")
		}
		t.subscribers[key] = op.subscriber
		return nil
	case subscriberActionRemove:
		if !ok {
			return errors.New("subscriber not found")
		}
		delete(t.subscribers, key)
		return nil
	}
	return errors.New("invalid subscriber action")
}

func (t *LocalTrack) loop() {
	defer t.cancelWG.Done()
	for {
		select {
		case <-t.ctx.Done():
			for _, v := range t.subscribers {
				v.Close()
			}
			return
		case op := <-t.subscriberCh:
			op.errCh <- t.manageSubscriber(op)
		case object := <-t.objectCh:
			for _, v := range t.subscribers {
				if err := v.WriteObject(object); err != nil {
					// TODO: Notify / remove subscriber?
					panic(err)
				}
			}
		case t.subscriberCountCh <- len(t.subscribers):
		}
	}
}

func (t *LocalTrack) subscribe(
	trackAlias uint64,
	subscribeID uint64,
	subscriber ObjectWriter,
) error {
	if subscriber == nil {
		return errors.New("subscriber MUST NOT be nil")
	}
	addOp := subscriberOp{
		action:       subscriberActionAdd,
		trackAlias:   trackAlias,
		subscriberID: subscribeID,
		subscriber:   subscriber,
		errCh:        make(chan error),
	}
	// TODO: Should this have a timeout or similar?
	t.subscriberCh <- addOp
	return <-addOp.errCh
}

func (t *LocalTrack) unsubscribe(trackAlias, subscribeID uint64) error {
	removeOp := subscriberOp{
		action:       subscriberActionRemove,
		trackAlias:   trackAlias,
		subscriberID: subscribeID,
		subscriber:   nil,
		errCh:        make(chan error),
	}
	t.subscriberCh <- removeOp
	return <-removeOp.errCh
}

// WriteObject adds an object with forwarding preference DATAGRAM
func (t *LocalTrack) WriteObject(ctx context.Context, o Object) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case t.objectCh <- o:
	}
	return nil
}

func (t *LocalTrack) Close() error {
	t.cancelCtx()
	t.cancelWG.Wait()
	return nil
}

func (t *LocalTrack) SubscriberCount() int {
	return <-t.subscriberCountCh
}
