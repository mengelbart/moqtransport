package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type Announcement struct {
	namespace  []string
	parameters wire.Parameters // TODO: This is unexported, need better API?
}

func (a *Announcement) Namespace() []string {
	return a.namespace
}

type AnnouncementResponseWriter interface {
	Accept()
	Reject(code uint64, reason string)
}

type AnnouncementHandler interface {
	HandleAnnouncement(*Transport, *Announcement, AnnouncementResponseWriter)
}

type AnnouncementHandlerFunc func(*Transport, *Announcement, AnnouncementResponseWriter)

func (f AnnouncementHandlerFunc) HandleAnnouncement(s *Transport, a *Announcement, arw AnnouncementResponseWriter) {
	f(s, a, arw)
}

type defaultAnnouncementResponseWriter struct {
	announcement Announcement
	transport    *Transport
}

func (a *defaultAnnouncementResponseWriter) Accept() {
	a.transport.acceptAnnouncement(a.announcement)
}

func (a *defaultAnnouncementResponseWriter) Reject(code uint64, reason string) {
	a.transport.rejectAnnouncement(a.announcement, code, reason)
}
