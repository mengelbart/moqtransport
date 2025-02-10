package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/stretchr/testify/assert"
)

func TestFetch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageFetch, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
		)
		assert.NoError(t, err)
		defer ct.Close()

		rt, err := ct.Fetch(context.Background(), 0, []string{"namespace"}, "track")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})
	t.Run("auth_error", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageFetch, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Reject(moqtransport.ErrorCodeFetchUnauthorized, "unauthorized"))
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
		)
		assert.NoError(t, err)
		defer ct.Close()

		rt, err := ct.Fetch(context.Background(), 0, []string{"namespace"}, "track")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
		assert.Nil(t, rt)
	})

	t.Run("receive_objects", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		publisherCh := make(chan moqtransport.FetchPublisher, 1)

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageFetch, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
						publisher, ok := w.(moqtransport.FetchPublisher)
						assert.True(t, ok)
						publisherCh <- publisher
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
		)
		assert.NoError(t, err)
		defer ct.Close()

		rt, err := ct.Fetch(context.Background(), 0, []string{"namespace"}, "track")
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

	t.Run("unsubscribe", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		publisherCh := make(chan moqtransport.Publisher, 1)

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageSubscribe, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
						publisher, ok := w.(moqtransport.Publisher)
						assert.True(t, ok)
						publisherCh <- publisher
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
		)
		assert.NoError(t, err)
		defer ct.Close()

		rt, err := ct.Subscribe(context.Background(), 0, 0, []string{"namespace"}, "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)

		var publisher moqtransport.Publisher
		select {
		case publisher = <-publisherCh:
		case <-time.After(time.Second):
			assert.FailNow(t, "timeout while waiting for publisher")
		}

		assert.NoError(t, rt.Close())

		time.Sleep(10 * time.Millisecond)

		p, err := publisher.OpenSubgroup(0, 0, 0)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unsubscribed")
		assert.Nil(t, p)
	})

	t.Run("subscribe_done", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		publisherCh := make(chan moqtransport.Publisher, 1)

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageSubscribe, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
						publisher, ok := w.(moqtransport.Publisher)
						assert.True(t, ok)
						publisherCh <- publisher
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
		)
		assert.NoError(t, err)
		defer ct.Close()

		rt, err := ct.Subscribe(context.Background(), 0, 0, []string{"namespace"}, "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)

		var publisher moqtransport.Publisher
		select {
		case publisher = <-publisherCh:
		case <-time.After(time.Second):
			assert.FailNow(t, "timeout while waiting for publisher")
		}

		err = publisher.CloseWithError(moqtransport.SubscribeStatusSubscriptionEnded, "done")
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		o, err := rt.ReadObject(ctx)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "done")
		assert.Nil(t, o)
	})
}
