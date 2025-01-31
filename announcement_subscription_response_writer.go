package moqtransport

type AnnouncementSubscriptionResponseWriter interface {
	Accept() error
	Reject(code uint64, reason string) error
}

type defaultAnnouncementSubscriptionResponseWriter struct {
	subscription AnnouncementSubscription
	transport    *Transport
}

func (a *defaultAnnouncementSubscriptionResponseWriter) Accept() error {
	return a.transport.acceptAnnouncementSubscription(a.subscription)
}

func (a *defaultAnnouncementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncementSubscription(a.subscription, code, reason)
}
