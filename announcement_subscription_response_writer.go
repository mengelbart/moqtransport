package moqtransport

type announcementSubscriptionResponseWriter struct {
	prefix  []string
	session *Session
}

func (a *announcementSubscriptionResponseWriter) Accept() error {
	return a.session.acceptAnnouncementSubscription(a.prefix)
}

func (a *announcementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	return a.session.rejectAnnouncementSubscription(a.prefix, code, reason)
}
