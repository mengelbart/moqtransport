package moqtransport

type announcementResponseWriter struct {
	namespace []string
	session   *Session
}

func (a *announcementResponseWriter) Accept() error {
	return a.session.acceptAnnouncement(a.namespace)
}

func (a *announcementResponseWriter) Reject(code uint64, reason string) error {
	return a.session.rejectAnnouncement(a.namespace, code, reason)
}
