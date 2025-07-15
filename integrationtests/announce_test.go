package integrationtests

import (
	"context"
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/stretchr/testify/assert"
)

func TestAnnounce(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageAnnounce, m.Method)
			assert.NotNil(t, w)
			assert.NoError(t, w.Accept())
		})
		_, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		err := ct.Announce(context.Background(), []string{"namespace"})
		assert.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m *moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageAnnounce, m.Method)
			assert.NotNil(t, w)
			assert.NoError(t, w.Reject(uint64(moqtransport.ErrorCodeAnnounceInternal), "expected error"))
		})
		_, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		err := ct.Announce(context.Background(), []string{"namespace"})
		assert.Error(t, err)
	})
}
