package moqtransport

type announcementSubscriptionResponseWriter struct {
	requestID uint64
	session   *Session
	handled   bool
}

func (a *announcementSubscriptionResponseWriter) Accept() error {
	a.handled = true
	return a.session.acceptAnnouncementSubscription(a.requestID)
}

func (a *announcementSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	a.handled = true
	return a.session.rejectAnnouncementSubscription(a.requestID, code, reason)
}
