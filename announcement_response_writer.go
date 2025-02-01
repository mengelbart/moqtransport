package moqtransport

type announcementResponseWriter struct {
	namespace []string
	transport *Transport
}

func (a *announcementResponseWriter) Accept() error {
	return a.transport.acceptAnnouncement(a.namespace)
}

func (a *announcementResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncement(a.namespace, code, reason)
}
