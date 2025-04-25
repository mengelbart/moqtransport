package moqtransport

type announcementSubscriptionResponseWriter struct {
	prefix  []string
	session *Session
	handled bool
}

// Session returns the session associated with this response writer
func (a *announcementSubscriptionResponseWriter) Session() *Session {
	return a.session
}

func (a *announcementSubscriptionResponseWriter) Accept() error {
	a.handled = true
	return a.session.acceptAnnouncementSubscription(a.prefix)
}

func (a *announcementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	a.handled = true
	return a.session.rejectAnnouncementSubscription(a.prefix, code, reason)
}
