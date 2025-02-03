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

func (c *callbacks) onMessage(m *Message) {
	if c.handler != nil {
		switch m.Method {
		case MethodSubscribe:
			c.handler.Handle(&subscriptionResponseWriter{
				id:         m.SubscribeID,
				trackAlias: m.TrackAlias,
				transport:  c.t,
			}, m)
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
