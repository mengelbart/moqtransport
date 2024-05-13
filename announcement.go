package moqtransport

type AnnouncementResponseWriter interface {
	Accept()
	Reject(code uint64, reason string)
}

type defaultAnnouncementResponseWriter struct {
	a *Announcement
	s *Session
}

func (a *defaultAnnouncementResponseWriter) Accept() {
	a.s.acceptAnnouncement(a.a)
}

func (a *defaultAnnouncementResponseWriter) Reject(code uint64, reason string) {
	a.s.rejectAnnouncement(a.a, code, reason)
}

type AnnouncementHandler interface {
	Handle(*Announcement, AnnouncementResponseWriter)
}

type AnnouncementHandlerFunc func(*Announcement, AnnouncementResponseWriter)

func (f AnnouncementHandlerFunc) Handle(a *Announcement, arw AnnouncementResponseWriter) {
	f(a, arw)
}

type Announcement struct {
	responseCh chan trackNamespacer
	namespace  string
	parameters parameters
}

func (a *Announcement) Namespace() string {
	return a.namespace
}
