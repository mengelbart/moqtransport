package moqtransport

import "io"

const (
	MethodSubscribe            = "SUBSCRIBE"
	MethodFetch                = "FETCH"
	MethodAnnounce             = "ANNOUNCE"
	MethodAnnounceCancel       = "ANNOUNCE_CANCEL"
	MethodUnannounce           = "UNANNOUNCE"
	MethodTrackStatusRequest   = "TRACK_STATUS_REQUEST"
	MethodTrackStatus          = "TRACK_STATUS"
	MethodGoAway               = "GO_AWAY"
	MethodSubscribeAnnounces   = "SUBSCRIBE_ANNOUNCES"
	MethodUnsubscribeAnnounces = "UNSUBSCRIBE_ANNOUNCES"
)

type Message struct {
	Method string

	Namespace []string
	Track     string

	Authorization string

	// TrackStatusMessage
	Status       uint64
	LastGroupID  uint64
	LastObjectID uint64

	// GoAway
	NewSessionURI string

	// Generic Errors
	ErrorCode    uint64
	ReasonPhrase string
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
	Handle(ResponseWriter, *Message)
}

type HandlerFunc func(ResponseWriter, *Message)

func (f HandlerFunc) Handle(rw ResponseWriter, r *Message) {
	f(rw, r)
}
