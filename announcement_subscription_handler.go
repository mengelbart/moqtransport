package moqtransport

type AnnouncementSubscriptionHandler interface {
	HandleAnnouncementSubscription(*Transport, AnnouncementSubscription, AnnouncementSubscriptionResponseWriter)
}

type AnnouncementSubscriptionHandlerFunc func(*Transport, AnnouncementSubscription, AnnouncementSubscriptionResponseWriter)

func (f AnnouncementSubscriptionHandlerFunc) HandleAnnouncementSubscription(t *Transport, as AnnouncementSubscription, asrw AnnouncementSubscriptionResponseWriter) {
	f(t, as, asrw)
}
