package moqtransport

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

// Message represents a message from the peer that can be handled by the
// application.
type Message struct {
	// Method describes the type of the message.
	Method string

	// RequestID is set if the message references a request.
	RequestID uint64

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
}

// ResponseWriter can be used to respond to messages that expect a response.
type ResponseWriter interface {
	// Accept sends an affirmative response to a message.
	Accept() error

	// Reject sends a negative response to a message.
	Reject(code uint64, reason string) error
}

// Publisher is the interface implemented by SubscribeResponseWriters
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

// Handler is the handler interface for non-specific  MoQ messages.
type Handler interface {
	Handle(ResponseWriter, *Message)
}

// HandlerFunc is a type that implements Handler.
type HandlerFunc func(ResponseWriter, *Message)

// Handle implements Handler.
func (f HandlerFunc) Handle(rw ResponseWriter, r *Message) {
	f(rw, r)
}

// SubcribeHandler is the handler interface for handling SUBSCRIBE messages.
type SubscribeHandler interface {
	HandleSubscribe(*SubscribeResponseWriter, *SubscribeMessage)
}

// SubscribeHandlerFunc is a type that implements SubscribeHandler.
type SubscribeHandlerFunc func(*SubscribeResponseWriter, *SubscribeMessage)

// HandleSubscribe implements SubscribeHandler.
func (f SubscribeHandlerFunc) HandleSubscribe(rw *SubscribeResponseWriter, m *SubscribeMessage) {
	f(rw, m)
}

// SubscribeUpdateHandler is the handler interface for handling SUBSCRIBE_UPDATE messages.
type SubscribeUpdateHandler interface {
	HandleSubscribeUpdate(*SubscribeUpdateMessage)
}

// SubscribeUpdateHandlerFunc is a type that implements SubscribeUpdateHandler.
type SubscribeUpdateHandlerFunc func(*SubscribeUpdateMessage)

// HandleSubscribeUpdate implements SubscribeUpdateHandler.
func (f SubscribeUpdateHandlerFunc) HandleSubscribeUpdate(m *SubscribeUpdateMessage) {
	f(m)
}
