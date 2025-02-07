package moqtransport

type announcementSubscriptionResponseWriter struct {
	prefix    []string
	transport *Transport
}

func (a *announcementSubscriptionResponseWriter) Accept() error {
	return a.transport.acceptAnnouncementSubscription(a.prefix)
}

func (a *announcementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncementSubscription(a.prefix, code, reason)
}
