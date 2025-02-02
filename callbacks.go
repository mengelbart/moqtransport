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

func (c *callbacks) onSubscription(r *Message, s *subscription) {
	c.handler.Handle(&subscriptionResponseWriter{
		subscription: s,
		transport:    c.t,
	}, r)
}

func (c *callbacks) onMessage(m *Message) {
	if c.handler != nil {
		switch m.Method {
		case MethodAnnounce:
			c.handler.Handle(&announcementResponseWriter{
				namespace: m.Namespace,
				transport: c.t,
			}, m)
		case MethodSubscribeAnnounces:
			c.handler.Handle(&announcementSubscriptionResponseWriter{
				subscription: announcementSubscription{
					namespace: m.Namespace,
					response:  make(chan announcementSubscriptionResponse),
				},
				transport: c.t,
			}, m)
		default:
			c.handler.Handle(nil, m)
		}
	}
	// TODO: Handle unhandled request
}
