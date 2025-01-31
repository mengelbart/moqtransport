package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type Announcement struct {
	Namespace  []string
	parameters wire.Parameters // TODO: This is unexported, need better API?
}

type AnnouncementResponseWriter interface {
	Accept() error
	Reject(code uint64, reason string) error
}

type AnnouncementHandler interface {
	HandleAnnouncement(*Transport, Announcement, AnnouncementResponseWriter)
}

type AnnouncementHandlerFunc func(*Transport, Announcement, AnnouncementResponseWriter)

func (f AnnouncementHandlerFunc) HandleAnnouncement(s *Transport, a Announcement, arw AnnouncementResponseWriter) {
	f(s, a, arw)
}

type defaultAnnouncementResponseWriter struct {
	announcement Announcement
	transport    *Transport
}

func (a *defaultAnnouncementResponseWriter) Accept() error {
	return a.transport.acceptAnnouncement(a.announcement)
}

func (a *defaultAnnouncementResponseWriter) Reject(code uint64, reason string) error {
	return a.transport.rejectAnnouncement(a.announcement, code, reason)
}
