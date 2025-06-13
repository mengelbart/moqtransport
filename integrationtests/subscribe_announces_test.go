package integrationtests

import (
	"context"
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/stretchr/testify/assert"
)

func TestSubscribeAnnounces(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sConn, cConn, cancel := connect(t)
		defer cancel()

		handler := moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, m moqtransport.Message) {
			assert.Equal(t, moqtransport.MessageSubscribeAnnounces, m.Method())
			assert.NotNil(t, w)
			assert.NoError(t, w.Accept())
		})
		_, _, _, ct, cancel := setup(t, sConn, cConn, handler)
		defer cancel()

		err := ct.SubscribeAnnouncements(context.Background(), []string{"test_prefix"})
		assert.NoError(t, err)
	})
}
