package integrationtests

import (
	"context"
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/stretchr/testify/assert"
)

func TestSubscribeAnnounces(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageSubscribeAnnounces, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
					},
				),
			),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(quicmoq.NewClient(cConn))
		assert.NoError(t, err)
		defer ct.Close()

		err = ct.SubscribeAnnouncements(context.Background(), []string{"test_prefix"})
		assert.NoError(t, err)
	})
}
