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
		case MessageSubscribe:
			c.handler.Handle(&subscriptionResponseWriter{
				id:         m.SubscribeID,
				trackAlias: m.TrackAlias,
				transport:  c.t,
			}, m)
		case MessageFetch:
			c.handler.Handle(&fetchResponseWriter{
				id:        m.SubscribeID,
				transport: c.t,
			}, m)
		case MessageAnnounce:
			c.handler.Handle(&announcementResponseWriter{
				namespace: m.Namespace,
				transport: c.t,
			}, m)
		case MessageSubscribeAnnounces:
			c.handler.Handle(&announcementSubscriptionResponseWriter{
				prefix:    m.Namespace,
				transport: c.t,
			}, m)
		default:
			c.handler.Handle(nil, m)
		}
	}
	// TODO: Handle unhandled request
}
