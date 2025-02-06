package integrationtests

import (
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/stretchr/testify/assert"
)

func TestHandshake(t *testing.T) {
	sConn, cConn, cancel := connect(t)
	defer cancel()

	st, err := moqtransport.NewTransport(quicmoq.NewServer(sConn))
	assert.NoError(t, err)
	defer st.Close()

	ct, err := moqtransport.NewTransport(quicmoq.NewClient(cConn))
	assert.NoError(t, err)
	defer ct.Close()
}
