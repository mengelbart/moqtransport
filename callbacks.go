package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type callbacks struct {
	t       *Transport
	handler Handler
}

// queueControlMessage implements sessionCallbacks.
func (c *callbacks) queueControlMessage(msg wire.ControlMessage) error {
	return c.t.queueCtrlMessage(msg)
}

// onProtocolViolation implements sessionCallbacks.
func (c *callbacks) onProtocolViolation(err ProtocolError) {
	c.t.destroy(err)
}

func (c *callbacks) onSubscription(r *Request, s *subscription) {
	c.handler.Handle(&subscriptionResponseWriter{
		subscription: s,
		transport:    c.t,
	}, r)
}

func (c *callbacks) onRequest(r *Request) {
	if c.handler != nil {
		switch r.Method {
		case MethodAnnounce:
			c.handler.Handle(&announcementResponseWriter{
				namespace: r.Namespace,
				transport: c.t,
			}, r)
		}
	}
	// TODO: Handle unhandled request
}
