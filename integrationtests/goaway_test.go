package integrationtests

import (
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type goAway struct {
	uri     string
	timeout time.Duration
}

func waitForGoAway(t *testing.T, received <-chan goAway) goAway {
	t.Helper()
	select {
	case g := <-received:
		return g
	case <-testContext(t).Done():
		require.FailNow(t, "timeout waiting for GOAWAY")
		return goAway{}
	}
}

func TestGoAwayRejectsLaterRequests(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		received := make(chan goAway, 1)
		clientHandler := &testHandler{
			onGoAway: func(uri string, timeout time.Duration) {
				received <- goAway{uri, timeout}
			},
		}
		serverHandler, _ := subscribeHandler(1)
		server, client := setup(t, tr, serverHandler, clientHandler)

		require.NoError(t, server.GoAway("https://example.com/moq", 3*time.Second))
		assert.Equal(t, goAway{"https://example.com/moq", 3 * time.Second}, waitForGoAway(t, received))

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.ErrorIs(t, err, &moqtransport.RequestError{Code: moqtransport.RequestErrorCodeGoingAway})
		assert.Nil(t, sub)

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestClientGoAway(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		received := make(chan goAway, 1)
		serverHandler := &testHandler{
			onGoAway: func(uri string, timeout time.Duration) {
				received <- goAway{uri, timeout}
			},
		}
		server, client := setup(t, tr, serverHandler, nil)

		require.ErrorIs(t, client.GoAway("https://example.com/moq", 0), moqtransport.ErrGoAwayURIFromClient)
		require.NoError(t, client.GoAway("", time.Second))
		assert.Equal(t, goAway{"", time.Second}, waitForGoAway(t, received))

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestRequestGoAway(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		server, client := setup(t, tr, handler, nil)

		received := make(chan goAway, 1)
		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack, moqtransport.WithGoAwayHandler(func(uri string, timeout time.Duration) {
			received <- goAway{uri, timeout}
		}))
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		require.NoError(t, r.GoAway("https://example.com/moq", 500*time.Millisecond))
		assert.Equal(t, goAway{"https://example.com/moq", 500 * time.Millisecond}, waitForGoAway(t, received))

		// The subscription keeps working until the publisher ends it.
		require.NoError(t, r.SendDatagram(1, 0, 128, false, []byte("payload")))
		o, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, []byte("payload"), readPayload(t, o))

		require.NoError(t, r.Close(moqtransport.PublishDoneStatusCodeGoingAway, "migrate"))
		_, err = sub.ReadObject(testContext(t))
		require.ErrorIs(t, err, &moqtransport.PublishDone{StatusCode: moqtransport.PublishDoneStatusCodeGoingAway})

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}

func TestSubscriberRequestGoAway(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		received := make(chan goAway, 1)
		handler := &testHandler{
			onSubscribe: func(r *moqtransport.IncomingSubscribeRequest) {
				r.OnGoAway(func(uri string, timeout time.Duration) {
					received <- goAway{uri, timeout}
				})
				r.Accept(1)
			},
		}
		server, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)

		require.ErrorIs(t, sub.GoAway("https://example.com/moq", 0), moqtransport.ErrGoAwayURIFromClient)
		require.NoError(t, sub.GoAway("", 250*time.Millisecond))
		assert.Equal(t, goAway{"", 250 * time.Millisecond}, waitForGoAway(t, received))

		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}
