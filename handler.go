package moqtransport

// Handler is the handler interface for non-specific  MoQ messages.
type Handler interface {
	HandleGoAway()
	HandleSubscribe(*IncomingSubscribeRequest)
}
