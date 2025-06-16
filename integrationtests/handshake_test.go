package integrationtests

import (
	"testing"
)

func TestHandshake(t *testing.T) {
	sConn, cConn, cancel := connect(t)
	defer cancel()

	_, _, cancel = setup(t, sConn, cConn, nil)
	defer cancel()
}
