package moqtransport

type announcementResponseWriter struct {
	namespace []string
	session   *Session
	handled   bool
}

// Session returns the session associated with this response writer
func (a *announcementResponseWriter) Session() *Session {
	return a.session
}

func (a *announcementResponseWriter) Accept() error {
	a.handled = true
	return a.session.acceptAnnouncement(a.namespace)
}

func (a *announcementResponseWriter) Reject(code uint64, reason string) error {
	a.handled = true
	return a.session.rejectAnnouncement(a.namespace, code, reason)
}
