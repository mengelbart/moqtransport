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

func (c *callbacks) onRequest(r *Request) {
	if c.handler != nil {
		switch r.Method {
		case MethodSubscribe:
			c.handler.Handle(&subscriptionResponseWriter{
				subscription: r.Subscription,
				transport:    c.t,
				localTrack:   nil,
			}, r)
		case MethodAnnounce:
			c.handler.Handle(&announcementResponseWriter{
				announcement: r.Announcement,
				transport:    c.t,
			}, r)
		}
	}
	// TODO: Handle unhandled request
}
