package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type announcementResponse struct {
	err error
}

type Announcement struct {
	Namespace  []string
	parameters wire.Parameters // TODO: This is unexported, need better API?

	response chan announcementResponse
}

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
