package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
)

func (t *Transport) logControlMessage(msg wire.ControlMessage, sending bool) {
	dir := "->"
	if !sending {
		dir = "<-"
	}
	t.logger.Debug(dir, msg.Type().String(), msg)
}
