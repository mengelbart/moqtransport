package integrationtests

import (
	"context"
	"testing"

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
}
