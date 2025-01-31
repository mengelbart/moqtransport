package moqtransport

type AnnouncementSubscriptionResponseWriter interface {
	Accept()
	Reject(code uint64, reason string)
}

type defaultAnnouncementSubscriptionResponseWriter struct {
	subscription AnnouncementSubscription
	transport    *Transport
}

func (a *defaultAnnouncementSubscriptionResponseWriter) Accept() {
	a.transport.acceptAnnouncementSubscription(a.subscription)
}

func (a *defaultAnnouncementSubscriptionResponseWriter) Reject(code uint64, reason string) {
	a.transport.rejectAnnouncementSubscription(a.subscription, code, reason)
}
