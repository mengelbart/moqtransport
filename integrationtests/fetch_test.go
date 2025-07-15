package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/stretchr/testify/assert"
)

func TestFetch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageFetch, m.Method)
			assert.NotNil(t, w)
			assert.NoError(t, w.Accept())
		})
		_, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		rt, err := ct.Fetch(context.Background(), []string{"namespace"}, "track")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})
	t.Run("auth_error", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageFetch, m.Method)
			assert.NotNil(t, w)
			assert.NoError(t, w.Reject(uint64(moqtransport.ErrorCodeFetchUnauthorized), "unauthorized"))
		})
		_, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		rt, err := ct.Fetch(context.Background(), []string{"namespace"}, "track")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
		assert.Nil(t, rt)
	})

	t.Run("receive_objects", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		publisherCh := make(chan moqtransport.FetchPublisher, 1)

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageFetch, m.Method)
			assert.NotNil(t, w)
			assert.NoError(t, w.Accept())
			publisher, ok := w.(moqtransport.FetchPublisher)
			assert.True(t, ok)
			publisherCh <- publisher
		})
		_, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		rt, err := ct.Fetch(context.Background(), []string{"namespace"}, "track")
		assert.NoError(t, err)
		assert.NotNil(t, rt)

		var publisher moqtransport.FetchPublisher
		select {
		case publisher = <-publisherCh:
		case <-time.After(time.Second):
			assert.FailNow(t, "timeout while waiting for publisher")
		}

		fs, err := publisher.FetchStream()
		assert.NoError(t, err)
		n, err := fs.WriteObject(1, 2, 3, 0, []byte("hello fetch"))
		assert.NoError(t, err)
		assert.Equal(t, 11, n)
		assert.NoError(t, fs.Close())

		ctx2, cancelCtx2 := context.WithTimeout(context.Background(), time.Second)
		defer cancelCtx2()

		o, err := rt.ReadObject(ctx2)
		assert.NoError(t, err)
		assert.Equal(t, &moqtransport.Object{
			GroupID:    1,
			SubGroupID: 2,
			ObjectID:   3,
			Payload:    []byte("hello fetch"),
		}, o)
	})
}
