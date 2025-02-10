package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/stretchr/testify/assert"
)

func TestSubscribe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageSubscribe, m.Method)
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

		rt, err := ct.Subscribe(context.Background(), 0, 0, []string{"namespace"}, "track", "auth")
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
						assert.Equal(t, moqtransport.MessageSubscribe, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Reject(moqtransport.ErrorCodeSubscribeUnauthorized, "unauthorized"))
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
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
		assert.Nil(t, rt)
	})
	t.Run("receive_objects", func(t *testing.T) {
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

		o1 := moqtransport.Object{
			GroupID:    0,
			SubGroupID: 0,
			ObjectID:   0,
			Payload:    []byte("hello world"),
		}
		assert.NoError(t, publisher.SendDatagram(o1))

		ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
		defer cancelCtx()

		o, err := rt.ReadObject(ctx)
		assert.NoError(t, err)
		assert.Equal(t, &o1, o)

		sg, err := publisher.OpenSubgroup(1, 0, 0)
		assert.NoError(t, err)
		n, err := sg.WriteObject(0, []byte("hello again"))
		assert.NoError(t, err)
		assert.Equal(t, 11, n)
		assert.NoError(t, sg.Close())

		ctx2, cancelCtx2 := context.WithTimeout(context.Background(), time.Second)
		defer cancelCtx2()

		o, err = rt.ReadObject(ctx2)
		assert.NoError(t, err)
		assert.Equal(t, &moqtransport.Object{
			GroupID:    1,
			SubGroupID: 0,
			ObjectID:   0,
			Payload:    []byte("hello again"),
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
