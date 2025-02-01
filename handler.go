package moqtransport

import "io"

const (
	MethodSubscribe         = "SUBSCRIBE"
	MethodFetch             = "FETCH"
	MethodAnnounce          = "ANNOUNCE"
	MethodUnannounce        = "UNANNOUNCE"
	MethodSubscribeAnnounce = "SUBSCRIBE_ANNOUNCE"
)

type Request struct {
	Method string

	Subscription             *Subscription
	Announcement             *Announcement
	AnnouncementSubscription *AnnouncementSubscription
	Unannouncement           *Unannouncement
}

type ResponseWriter interface {
	Accept() error
	Reject(code uint64, reason string) error
}

type Publisher interface {
	SendDatagram(Object) error
	OpenSubgroup(groupID uint64, priority uint8) (*Subgroup, error)
	io.Closer
}

type Handler interface {
	Handle(ResponseWriter, *Request)
}

type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) Handle(rw ResponseWriter, r *Request) {
	f(rw, r)
}
