package moqtransport

type announcementSubscriptionResponseWriter struct {
	subscription AnnouncementSubscription
	transport    *Transport
}

func (a *announcementSubscriptionResponseWriter) Accept() error {
	return a.transport.acceptAnnouncementSubscription(a.subscription)
}

func (a *announcementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncementSubscription(a.subscription, code, reason)
}
