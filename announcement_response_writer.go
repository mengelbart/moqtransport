package moqtransport

type announcementResponseWriter struct {
	announcement *Announcement
	transport    *Transport
}

func (a *announcementResponseWriter) Accept() error {
	return a.transport.acceptAnnouncement(a.announcement)
}

func (a *announcementResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncement(a.announcement, code, reason)
}
