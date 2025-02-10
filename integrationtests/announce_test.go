package integrationtests

import (
	"context"
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/stretchr/testify/assert"
)

func TestAnnounce(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageAnnounce, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Accept())
					},
				),
			),
			moqtransport.DisableDatagrams(),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
			moqtransport.DisableDatagrams(),
		)
		assert.NoError(t, err)
		defer ct.Close()

		err = ct.Announce(context.Background(), []string{"namespace"})
		assert.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		st, err := moqtransport.NewTransport(
			quicmoq.NewServer(sConn),
			moqtransport.OnRequest(
				moqtransport.HandlerFunc(
					func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
						assert.Equal(t, moqtransport.MessageAnnounce, m.Method)
						assert.NotNil(t, w)
						assert.NoError(t, w.Reject(moqtransport.ErrorCodeAnnouncementInternalError, "expected error"))
					},
				),
			),
			moqtransport.DisableDatagrams(),
		)
		assert.NoError(t, err)
		defer st.Close()

		ct, err := moqtransport.NewTransport(
			quicmoq.NewClient(cConn),
			moqtransport.DisableDatagrams(),
		)
		assert.NoError(t, err)
		defer ct.Close()

		err = ct.Announce(context.Background(), []string{"namespace"})
		assert.Error(t, err)
	})
}
