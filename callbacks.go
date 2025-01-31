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
func (c *callbacks) onSubscription(s Subscription) {
	if c.subscriptionHandler != nil {
		c.subscriptionHandler.HandleSubscription(c.t, s, &defaultSubscriptionResponseWriter{
			subscription: s,
			transport:    c.t,
		})
	} else {
		c.t.session.RejectSubscription(s, SubscribeErrorTrackDoesNotExist, "track not found")
	}
}

// onAnnouncement implements sessionCallbacks.
func (c *callbacks) onAnnouncement(a Announcement) {
	if c.announcementHandler != nil {
		c.announcementHandler.HandleAnnouncement(c.t, a, &defaultAnnouncementResponseWriter{
			announcement: a,
			transport:    c.t,
		})
	} else {
		c.t.session.RejectAnnouncement(a, AnnouncementRejected, "session does not accept announcements")
	}
}

// onAnnouncementSubscription implements sessionCallbacks.
func (c *callbacks) onAnnouncementSubscription(AnnouncementSubscription) {
	panic("unimplemented")
}
