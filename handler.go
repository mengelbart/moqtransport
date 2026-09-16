package moqtransport

import "time"

// Handler is the handler interface for non-specific  MoQ messages.
type Handler interface {
	// HandleGoAway is called when the peer sent GOAWAY on the control stream.
	HandleGoAway(uri string, timeout time.Duration)
	HandleSubscribe(*IncomingSubscribeRequest)
}
