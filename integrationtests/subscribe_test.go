package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testNamespace = [][]byte{[]byte("ns"), []byte("sub")}
	testTrack     = "track"
)

func TestSubscribeHandlerReceivesRequest(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		_, client := setup(t, tr, handler, nil)

		_, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)

		r := waitForRequest(t, requests)
		assert.Equal(t, testNamespace, r.Namespace())
		assert.Equal(t, []byte(testTrack), r.Name())
	})
}

func TestSubscribeReceiveObjects(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		_, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		sg, err := r.OpenSubgroup(7, 3, 42)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("first"))
		w, err := sg.BufferObject(2)
		require.NoError(t, err)
		_, err = w.Write([]byte("sec"))
		require.NoError(t, err)
		_, err = w.Write([]byte("ond"))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		require.NoError(t, sg.Close())

		o, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(7), o.GroupID)
		assert.Equal(t, uint64(3), o.SubGroupID)
		assert.Equal(t, uint64(0), o.ObjectID)
		assert.Equal(t, uint8(42), o.PublisherPriority)
		assert.Equal(t, moqtransport.ObjectForwardingPreferenceSubgroup, o.ForwardingPreference)
		assert.Equal(t, moqtransport.ObjectStatusNormal, o.Status)
		assert.Equal(t, []byte("first"), readPayload(t, o))

		o, err = sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(7), o.GroupID)
		assert.Equal(t, uint64(3), o.SubGroupID)
		assert.Equal(t, uint64(2), o.ObjectID)
		assert.Equal(t, []byte("second"), readPayload(t, o))
	})
}

func TestSubscribeReceiveDatagrams(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		_, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		require.NoError(t, r.SendDatagram(7, 0, 42, true, []byte("hello")))
		require.NoError(t, r.SendDatagramStatus(7, 1, 42, moqtransport.ObjectStatusEndOfTrack))

		o, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, moqtransport.ObjectForwardingPreferenceDatagram, o.ForwardingPreference)
		assert.Equal(t, uint64(7), o.GroupID)
		assert.Equal(t, uint64(0), o.ObjectID)
		assert.Equal(t, uint8(42), o.PublisherPriority)
		assert.True(t, o.EndOfGroup)
		assert.Equal(t, moqtransport.ObjectStatusNormal, o.Status)
		assert.Equal(t, []byte("hello"), readPayload(t, o))

		o, err = sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, moqtransport.ObjectForwardingPreferenceDatagram, o.ForwardingPreference)
		assert.Equal(t, uint64(7), o.GroupID)
		assert.Equal(t, uint64(1), o.ObjectID)
		assert.Equal(t, uint8(42), o.PublisherPriority)
		assert.False(t, o.EndOfGroup)
		assert.Equal(t, moqtransport.ObjectStatusEndOfTrack, o.Status)
		assert.Empty(t, readPayload(t, o))
	})
}

func TestSubscribeMultipleTracks(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		requests := make(chan *moqtransport.IncomingSubscribeRequest, 2)
		var nextAlias uint64
		handler := &testHandler{
			onSubscribe: func(r *moqtransport.IncomingSubscribeRequest) {
				nextAlias++
				r.Accept(nextAlias)
				requests <- r
			},
		}
		_, client := setup(t, tr, handler, nil)

		subA, err := client.Subscribe(testContext(t), testNamespace, "a")
		require.NoError(t, err)
		rA := waitForRequest(t, requests)
		subB, err := client.Subscribe(testContext(t), testNamespace, "b")
		require.NoError(t, err)
		rB := waitForRequest(t, requests)
		assert.Equal(t, []byte("a"), rA.Name())
		assert.Equal(t, []byte("b"), rB.Name())

		sgA, err := rA.OpenSubgroup(1, 0, 0)
		require.NoError(t, err)
		writeObject(t, sgA, 0, []byte("for a"))
		require.NoError(t, sgA.Close())
		sgB, err := rB.OpenSubgroup(2, 0, 0)
		require.NoError(t, err)
		writeObject(t, sgB, 0, []byte("for b"))
		require.NoError(t, sgB.Close())

		o, err := subB.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(2), o.GroupID)
		assert.Equal(t, []byte("for b"), readPayload(t, o))

		o, err = subA.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(1), o.GroupID)
		assert.Equal(t, []byte("for a"), readPayload(t, o))
	})
}

func TestSubscribeRejectReturnsError(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler := &testHandler{
			onSubscribe: func(r *moqtransport.IncomingSubscribeRequest) {
				r.Reject(moqtransport.RequestErrorCodeDoesNotExist, "unknown")
			},
		}
		server, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.ErrorIs(t, err, &moqtransport.RequestError{Code: moqtransport.RequestErrorCodeDoesNotExist})
		assert.ErrorContains(t, err, "unknown")
		assert.Nil(t, sub)

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestSubgroupResetDoesNotCloseSession(t *testing.T) {
	t.Skip("readDataStream closes the session on a stream reset")
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		server, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		sg, err := r.OpenSubgroup(0, 0, 0)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("payload"))
		o, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, []byte("payload"), readPayload(t, o))

		sg.Reset(moqtransport.StreamResetErrorCodeCancelled)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err = sub.ReadObject(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestPublishDoneEndsSubscription(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		server, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		sg, err := r.OpenSubgroup(0, 0, 0)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("first"))
		require.NoError(t, sg.Close())
		sg, err = r.OpenSubgroup(1, 0, 0)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("second"))
		require.NoError(t, sg.Close())
		require.NoError(t, r.Close(moqtransport.PublishDoneStatusCodeTrackEnded, "done"))

		// The subgroups are on separate streams, so their order is not fixed.
		var payloads []string
		for range 2 {
			o, err := sub.ReadObject(testContext(t))
			require.NoError(t, err)
			payloads = append(payloads, string(readPayload(t, o)))
		}
		assert.ElementsMatch(t, []string{"first", "second"}, payloads)

		_, err = sub.ReadObject(testContext(t))
		require.ErrorIs(t, err, &moqtransport.PublishDone{StatusCode: moqtransport.PublishDoneStatusCodeTrackEnded})
		var done *moqtransport.PublishDone
		require.ErrorAs(t, err, &done)
		assert.Equal(t, uint64(2), done.StreamCount)
		assert.Equal(t, "done", done.Reason)

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestSubscriberCloseCancelsPublisher(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		server, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		sg, err := r.OpenSubgroup(0, 0, 0)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("first"))
		o, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, []byte("first"), readPayload(t, o))

		require.NoError(t, sub.Close())

		select {
		case <-r.Context().Done():
		case <-testContext(t).Done():
			require.FailNow(t, "timeout waiting for publisher to see the cancellation")
		}
		assert.Error(t, context.Cause(r.Context()))
		assert.NoError(t, context.Cause(server.Context()))
		assert.NoError(t, context.Cause(client.Context()))

		_, err = r.OpenSubgroup(1, 0, 0)
		assert.Error(t, err)
		assert.Error(t, r.Close(moqtransport.PublishDoneStatusCodeTrackEnded, ""))
	})
}
