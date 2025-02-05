package moqtransport

import "io"

// Common Message types. Handlers can react to any of these messages.
const (
	MessageSubscribe            = "SUBSCRIBE"
	MessageFetch                = "FETCH"
	MessageAnnounce             = "ANNOUNCE"
	MessageAnnounceCancel       = "ANNOUNCE_CANCEL"
	MessageUnannounce           = "UNANNOUNCE"
	MessageTrackStatusRequest   = "TRACK_STATUS_REQUEST"
	MessageTrackStatus          = "TRACK_STATUS"
	MessageGoAway               = "GO_AWAY"
	MessageSubscribeAnnounces   = "SUBSCRIBE_ANNOUNCES"
	MessageUnsubscribeAnnounces = "UNSUBSCRIBE_ANNOUNCES"
)

type Message struct {
	Method string

	SubscribeID uint64
	TrackAlias  uint64

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
