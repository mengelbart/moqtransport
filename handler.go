package moqtransport

// Common Message types. Handlers can react to any of these messages.
const (
	MessageSubscribe            = "SUBSCRIBE"
	MessageSubscribeUpdate      = "SUBSCRIBE_UPDATE"
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

// Message represents a message from the peer that can be handled by the
// application.
type Message struct {
	// Method describes the type of the message.
	Method string

	// RequestID is set if the message references a request.
	RequestID uint64
	// TrackAlias corresponding to the subscription.
	TrackAlias uint64

	// Namespace is set if the message references a namespace.
	Namespace []string
	// Track is set if the message references a track.
	Track string

	// Authorization
	Authorization string

	// NewSessionURI is set in a GoAway message and points to a URI that can be
	// used to setup a new session before closing the current session.
	NewSessionURI string

	// ErrorCode is set if the message is an error message.
	ErrorCode uint64
	// ReasonPhrase is set if the message is an error message.
	ReasonPhrase string

	// SUBSCRIBE_UPDATE specific fields
	// SubscriberPriority is the new priority for the subscription (0-255, lower is higher priority)
	SubscriberPriority uint8
	// StartGroup and StartObject define the new start location (can only move forward)
	StartGroup  uint64
	StartObject uint64
	// EndGroup defines the new end location (can only move backward)
	EndGroup uint64
	// Forward controls whether to pause (0) or forward (1) object delivery
	Forward uint8
}

// ResponseWriter can be used to respond to messages that expect a response.
type ResponseWriter interface {
	// Accept sends an affirmative response to a message.
	Accept() error

	// Reject sends a negative response to a message.
	Reject(code uint64, reason string) error
}

// Publisher is the interface implemented by ResponseWriters of Subscribe
// messages.
type Publisher interface {
	// SendDatagram sends an object in a datagram.
	SendDatagram(Object) error

	// OpenSubgroup opens and returns a new subgroup.
	OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error)

	// CloseWithError closes the track and sends SUBSCRIBE_DONE with code and
	// reason.
	CloseWithError(code uint64, reason string) error
}

// FetchPublisher is the interface implemented by ResponseWriters of Fetch
// messages.
type FetchPublisher interface {
	// OpenFetchStream opens and returns a new fetch stream.
	FetchStream() (*FetchStream, error)
}

// StatusRequestHandler is the interface implemented by ResponseWriters of
// TrackStatusRequest messages. The first call to Accept sends the response.
// Calling Reject sets the status to "track does not exist" and then calls
// Accept. Reject ignores the errorCode and reasonPhrase. Applications are
// responsible for following the ruls of track status messages.
type StatusRequestHandler interface {
	// SetStatus sets the status for the response. Call this before calling
	// Accept.
	SetStatus(statusCode, lastGroupID, lastObjectID uint64)
}

// A Handler responds to MoQ messages.
type Handler interface {
	Handle(ResponseWriter, *Message)
}

// HandlerFunc is a type that implements Handler.
type HandlerFunc func(ResponseWriter, *Message)

// Handle implements Handler.
func (f HandlerFunc) Handle(rw ResponseWriter, r *Message) {
	f(rw, r)
}
