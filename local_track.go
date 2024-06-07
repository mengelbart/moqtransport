package moqtransport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
)

type subscriberID int

func (id *subscriberID) next() subscriberID {
	*id += 1
	return *id
}

type subscriberAction bool

const (
	subscriberActionAdd    subscriberAction = true
	subscriberActionRemove subscriberAction = false
)

type subscribeOp struct {
	action       subscriberAction
	subscriberID subscriberID
	subscriber   ObjectWriter
	resultCh     chan subscribeOpResult
}

type subscribeOpResult struct {
	id  subscriberID
	err error
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
	logger    *slog.Logger
	ID        uint64
	Namespace string
	Name      string

	cancelCtx context.CancelFunc
	cancelWG  sync.WaitGroup
	ctx       context.Context

	subscriberCh      chan subscribeOp
	subscribers       map[subscriberID]ObjectWriter
	objectCh          chan Object
	subscriberCountCh chan int

	nextID subscriberID
}

// NewLocalTrack creates a new LocalTrack
func NewLocalTrack(id uint64, namespace, trackname string) *LocalTrack {
	ctx, cancelCtx := context.WithCancel(context.Background())
	lt := &LocalTrack{
		logger:            defaultLogger.WithGroup("MOQ_LOCAL_TRACK").With("id", id, "namespace", namespace, "trackname", trackname),
		ID:                id,
		Namespace:         namespace,
		Name:              trackname,
		cancelCtx:         cancelCtx,
		cancelWG:          sync.WaitGroup{},
		ctx:               ctx,
		subscriberCh:      make(chan subscribeOp),
		subscribers:       map[subscriberID]ObjectWriter{},
		objectCh:          make(chan Object),
		subscriberCountCh: make(chan int),
		nextID:            0,
	}
	lt.cancelWG.Add(1)
	go lt.loop()
	return lt
}

func (t *LocalTrack) manageSubscriber(op subscribeOp) (subscriberID, error) {
	_, ok := t.subscribers[op.subscriberID]
	switch op.action {
	case subscriberActionAdd:
		id := t.nextID.next()
		t.subscribers[id] = op.subscriber
		return id, nil
	case subscriberActionRemove:
		if !ok {
			return op.subscriberID, errors.New("subscriber not found")
		}
		delete(t.subscribers, op.subscriberID)
		return op.subscriberID, nil
	}
	return 0, errors.New("invalid subscriber action")
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
			id, err := t.manageSubscriber(op)
			op.resultCh <- subscribeOpResult{
				id:  id,
				err: err,
			}
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
		return 0, errors.New("subscriber MUST NOT be nil")
	}
	addOp := subscribeOp{
		action:     subscriberActionAdd,
		subscriber: subscriber,
		resultCh:   make(chan subscribeOpResult),
	}
	// TODO: Should this have a timeout or similar?
	t.subscriberCh <- addOp
	res := <-addOp.resultCh
	return res.id, res.err
}

func (t *LocalTrack) unsubscribe(id subscriberID) error {
	removeOp := subscribeOp{
		action:       subscriberActionRemove,
		subscriberID: id,
		subscriber:   nil,
		resultCh:     make(chan subscribeOpResult),
	}
	t.subscriberCh <- removeOp
	res := <-removeOp.resultCh
	return res.err
}

// WriteObject adds an object to the track
func (t *LocalTrack) WriteObject(ctx context.Context, o Object) error {
	t.logger.Info("write object", "object", o)
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
