package moqtransport

type announcementResponseWriter struct {
	requestID uint64
	session   *Session
	handled   bool
}

func (a *announcementResponseWriter) Accept() error {
	a.handled = true
	return a.session.acceptAnnouncement(a.requestID)
}

func (a *announcementResponseWriter) Reject(code uint64, reason string) error {
	a.handled = true
	return a.session.rejectAnnouncement(a.requestID, code, reason)
}
