package integrationtests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseSessionTearsDownPeer(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		server, client := setup(t, tr, nil, nil)

		client.CloseWithError(0, "")

		select {
		case <-server.Context().Done():
		case <-testContext(t).Done():
			require.FailNow(t, "timeout waiting for server session to close")
		}
		assert.Error(t, context.Cause(server.Context()))
	})
}

func TestUnreadPayloadIsSkipped(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		handler, requests := subscribeHandler(1)
		_, client := setup(t, tr, handler, nil)

		sub, err := client.Subscribe(testContext(t), testNamespace, testTrack)
		require.NoError(t, err)
		r := waitForRequest(t, requests)

		sg, err := r.OpenSubgroup(0, 0, 0)
		require.NoError(t, err)
		writeObject(t, sg, 0, []byte("first"))
		writeObject(t, sg, 1, []byte("second"))
		require.NoError(t, sg.Close())

		first, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(0), first.ObjectID)

		second, err := sub.ReadObject(testContext(t))
		require.NoError(t, err)
		assert.Equal(t, uint64(1), second.ObjectID)
		assert.Equal(t, []byte("second"), readPayload(t, second))
	})
}
