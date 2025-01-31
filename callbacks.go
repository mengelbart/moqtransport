package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type callbacks struct {
	t                               *Transport
	subscriptionHandler             SubscriptionHandler
	announcementHandler             AnnouncementHandler
	announcementSubscriptionHandler AnnouncementSubscriptionHandler
}

// queueControlMessage implements sessionCallbacks.
func (c *callbacks) queueControlMessage(msg wire.ControlMessage) error {
	return c.t.queueCtrlMessage(msg)
}

// onSubscription implements sessionCallbacks.
func (c *callbacks) onSubscription(s Subscription) bool {
	if c.subscriptionHandler != nil {
		c.subscriptionHandler.HandleSubscription(c.t, s, &defaultSubscriptionResponseWriter{
			subscription: s,
			transport:    c.t,
		})
		return true
	}
	return false
}

// onAnnouncement implements sessionCallbacks.
func (c *callbacks) onAnnouncement(a Announcement) bool {
	if c.announcementHandler != nil {
		c.announcementHandler.HandleAnnouncement(c.t, a, &defaultAnnouncementResponseWriter{
			announcement: a,
			transport:    c.t,
		})
		return true
	}
	return false
}

// onAnnouncementSubscription implements sessionCallbacks.
func (c *callbacks) onAnnouncementSubscription(AnnouncementSubscription) bool {
	panic("unimplemented")
}
