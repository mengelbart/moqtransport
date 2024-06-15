package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type Announcement struct {
	responseCh chan trackNamespacer
	namespace  string
	parameters wire.Parameters // TODO: This is unexported, need better API?
}

func (a *Announcement) Namespace() string {
	return a.namespace
}

type AnnouncementResponseWriter interface {
	Accept()
	Reject(code uint64, reason string)
}

type AnnouncementHandler interface {
	HandleAnnouncement(*Session, *Announcement, AnnouncementResponseWriter)
}

type AnnouncementHandlerFunc func(*Session, *Announcement, AnnouncementResponseWriter)

func (f AnnouncementHandlerFunc) HandleAnnouncement(s *Session, a *Announcement, arw AnnouncementResponseWriter) {
	f(s, a, arw)
}

type defaultAnnouncementResponseWriter struct {
	announcement *Announcement
	session      *Session
}

func (a *defaultAnnouncementResponseWriter) Accept() {
	a.session.acceptAnnouncement(a.announcement)
}

func (a *defaultAnnouncementResponseWriter) Reject(code uint64, reason string) {
	a.session.rejectAnnouncement(a.announcement, code, reason)
}
