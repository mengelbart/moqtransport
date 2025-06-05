package moqtransport

import (
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
)

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
	// TrackAlias corresponding to the subscription.
	TrackAlias uint64

	// Namespace is set if the message references a namespace.
	Namespace []string
	// Track is set if the message references a track.
	Track string

	// Authorization token should be an object, see 8.2.1.1
	Authorization string

	// Subscribe message specific fields
	// SubscriberPriority indicates the delivery priority (0-255, higher is more important)
	SubscriberPriority uint8
	// GroupOrder indicates group ordering preference: 0=None, 1=Ascending, 2=Descending
	GroupOrder uint8
	// Forward indicates forward preference: 0=No, 1=Yes
	Forward uint8
	// FilterType specifies the subscription filter type
	FilterType wire.FilterType
	// StartLocation specifies the start position for absolute filters
	StartLocation *wire.Location
	// EndGroup specifies the end group for range filters
	EndGroup *uint64
	// Parameters contains the full parameter list from the subscribe message
	Parameters wire.KVPList

	// NewSessionURI is set in a GoAway message and points to a URI that can be
	// used to setup a new session before closing the current session.
	NewSessionURI string

	// ErrorCode is set if the message is an error message.
	ErrorCode uint64
	// ReasonPhrase is set if the message is an error message.
	ReasonPhrase string
}

// GetDeliveryTimeout extracts the delivery timeout parameter if present.
func (m *Message) GetDeliveryTimeout() (time.Duration, bool) {
	for _, param := range m.Parameters {
		if param.Type == wire.DeliveryTimeoutParameterKey {
			return time.Duration(param.ValueVarInt) * time.Millisecond, true
		}
	}
	return 0, false
}

// GetMaxCacheDuration extracts the max cache duration parameter if present.
func (m *Message) GetMaxCacheDuration() (time.Duration, bool) {
	for _, param := range m.Parameters {
		if param.Type == wire.MaxCacheDurationParameterKey && len(param.ValueBytes) > 0 {
			// Parse duration from bytes (implementation depends on format)
			// For now, return zero duration
			return 0, true
		}
	}
	return 0, false
}

// GetParameter extracts a custom parameter by key.
func (m *Message) GetParameter(key uint64) (wire.KeyValuePair, bool) {
	for _, param := range m.Parameters {
		if param.Type == key {
			return param, true
		}
	}
	return wire.KeyValuePair{}, false
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

// SubscribeResponseWriter is the enhanced interface implemented by ResponseWriters of Subscribe
// messages, providing full control over subscription response parameters.
type SubscribeResponseWriter interface {
	ResponseWriter

	// AcceptWithOptions accepts the subscription with custom response options.
	AcceptWithOptions(opts *SubscribeOkOptions) error
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
