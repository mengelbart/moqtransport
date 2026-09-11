package integrationtests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandshake(t *testing.T) {
	forEachTransport(t, func(t *testing.T, tr transport) {
		server, client := setup(t, tr, nil, nil)

		if tr.name == "quic" {
			require.Eventually(t, func() bool {
				return server.Path() == testPath
			}, testTimeout, 10*time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
			assert.Empty(t, server.Path())
		}
		assert.NoError(t, server.Context().Err())
		assert.NoError(t, client.Context().Err())
	})
}
