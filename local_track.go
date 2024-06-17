package moqtransport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type subscriberID int

func (id *subscriberID) next() subscriberID {
	*id += 1
	return *id
}

type addSubscriberOp struct {
	subscriber ObjectWriter
	resultCh   chan subscriberID
}

type removeSubscriberOp struct {
	subscriberID subscriberID
}

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceDatagram ObjectForwardingPreference = iota
	ObjectForwardingPreferenceStream
	ObjectForwardingPreferenceStreamGroup
	ObjectForwardingPreferenceStreamTrack
)

func objectForwardingPreferenceFromMessageType(t wire.ObjectMessageType) ObjectForwardingPreference {
	switch t {
	case wire.ObjectDatagramMessageType:
		return ObjectForwardingPreferenceDatagram
	case wire.ObjectStreamMessageType:
		return ObjectForwardingPreferenceStream
	case wire.StreamHeaderTrackMessageType:
		return ObjectForwardingPreferenceStreamTrack
	case wire.StreamHeaderGroupMessageType:
		return ObjectForwardingPreferenceStreamGroup
	}
	panic("invalid object message type")
}

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
	logger    *slog.Logger
	Namespace string
	Name      string

	cancelCtx context.CancelFunc
	cancelWG  sync.WaitGroup
	ctx       context.Context

	addSubscriberCh    chan addSubscriberOp
	removeSubscriberCh chan removeSubscriberOp
	subscribers        map[subscriberID]ObjectWriter
	objectCh           chan Object
	subscriberCountCh  chan int

	nextID subscriberID
}

// NewLocalTrack creates a new LocalTrack
func NewLocalTrack(namespace, trackname string) *LocalTrack {
	ctx, cancelCtx := context.WithCancel(context.Background())
	lt := &LocalTrack{
		logger:             defaultLogger.WithGroup("MOQ_LOCAL_TRACK").With("namespace", namespace, "trackname", trackname),
		Namespace:          namespace,
		Name:               trackname,
		cancelCtx:          cancelCtx,
		cancelWG:           sync.WaitGroup{},
		ctx:                ctx,
		addSubscriberCh:    make(chan addSubscriberOp),
		removeSubscriberCh: make(chan removeSubscriberOp),
		subscribers:        map[subscriberID]ObjectWriter{},
		objectCh:           make(chan Object),
		subscriberCountCh:  make(chan int),
		nextID:             0,
	}
	lt.cancelWG.Add(1)
	go lt.loop()
	return lt
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
		case op := <-t.addSubscriberCh:
			id := t.nextID.next()
			t.subscribers[id] = op.subscriber
			op.resultCh <- id
		case rem := <-t.removeSubscriberCh:
			delete(t.subscribers, rem.subscriberID)
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
	subscriber ObjectWriter,
) (subscriberID, error) {
	if subscriber == nil {
		return 0, errors.New("nil subscriber")
	}
	addOp := addSubscriberOp{
		subscriber: subscriber,
		resultCh:   make(chan subscriberID),
	}
	// TODO: Should this have a timeout or similar?
	t.addSubscriberCh <- addOp
	res := <-addOp.resultCh
	return res, nil
}

func (t *LocalTrack) unsubscribe(id subscriberID) {
	removeOp := removeSubscriberOp{
		subscriberID: id,
	}
	t.removeSubscriberCh <- removeOp
}

// WriteObject adds an object to the track
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
