package moqtransport

import (
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
)

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

// Message represents a message from the peer that can be handled by the application.
type Message interface {
	// Method returns the type of the message.
	Method() string
	
	// RequestID returns the request ID if the message references a request.
	RequestID() uint64
}

// SubscribeMessage represents a SUBSCRIBE message from the peer.
type SubscribeMessage struct {
	RequestID_ uint64
	TrackAlias uint64
	Namespace  []string
	Track      string
	
	// Authorization token should be an object, see 8.2.1.1
	Authorization string
	
	// Subscribe message specific fields
	SubscriberPriority uint8        // Delivery priority (0-255, higher is more important)
	GroupOrder         uint8        // Group ordering preference: 0=None, 1=Ascending, 2=Descending
	Forward            uint8        // Forward preference: 0=No, 1=Yes
	FilterType         FilterType // Subscription filter type
	StartLocation      *Location  // Start position for absolute filters
	EndGroup          *uint64     // End group for range filters
	Parameters        KVPList     // Full parameter list from the subscribe message
}

// Method returns the message type.
func (m *SubscribeMessage) Method() string {
	return MessageSubscribe
}

// RequestID returns the request ID.
func (m *SubscribeMessage) RequestID() uint64 {
	return m.RequestID_
}

// GetDeliveryTimeout extracts the delivery timeout parameter if present.
func (m *SubscribeMessage) GetDeliveryTimeout() (time.Duration, bool) {
	for _, param := range m.Parameters {
		if param.Type == wire.DeliveryTimeoutParameterKey {
			return time.Duration(param.ValueVarInt) * time.Millisecond, true
		}
	}
	return 0, false
}

// GetMaxCacheDuration extracts the max cache duration parameter if present.
func (m *SubscribeMessage) GetMaxCacheDuration() (time.Duration, bool) {
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
func (m *SubscribeMessage) GetParameter(key uint64) (KeyValuePair, bool) {
	for _, param := range m.Parameters {
		if param.Type == key {
			return param, true
		}
	}
	return KeyValuePair{}, false
}

// SubscribeUpdateMessage represents a SUBSCRIBE_UPDATE message from the peer.
type SubscribeUpdateMessage struct {
	RequestID_         uint64
	SubscriberPriority uint8           // New delivery priority
	Forward            uint8           // New forward preference
	StartLocation      Location   // New start position for the subscription
	EndGroup           uint64     // New end group for the subscription
	Parameters         KVPList    // Custom parameters for the update
}

// Method returns the message type.
func (m *SubscribeUpdateMessage) Method() string {
	return MessageSubscribeUpdate
}

// RequestID returns the request ID.
func (m *SubscribeUpdateMessage) RequestID() uint64 {
	return m.RequestID_
}

// GetDeliveryTimeout extracts the delivery timeout parameter if present.
func (m *SubscribeUpdateMessage) GetDeliveryTimeout() (time.Duration, bool) {
	for _, param := range m.Parameters {
		if param.Type == wire.DeliveryTimeoutParameterKey {
			return time.Duration(param.ValueVarInt) * time.Millisecond, true
		}
	}
	return 0, false
}

// GetParameter extracts a custom parameter by key.
func (m *SubscribeUpdateMessage) GetParameter(key uint64) (KeyValuePair, bool) {
	for _, param := range m.Parameters {
		if param.Type == key {
			return param, true
		}
	}
	return KeyValuePair{}, false
}

// AnnounceMessage represents an ANNOUNCE message from the peer.
type AnnounceMessage struct {
	RequestID_  uint64
	Namespace   []string
	Parameters  wire.KVPList // Parameters from the announce message
}

// Method returns the message type.
func (m *AnnounceMessage) Method() string {
	return MessageAnnounce
}

// RequestID returns the request ID.
func (m *AnnounceMessage) RequestID() uint64 {
	return m.RequestID_
}

// GetParameter extracts a custom parameter by key.
func (m *AnnounceMessage) GetParameter(key uint64) (wire.KeyValuePair, bool) {
	for _, param := range m.Parameters {
		if param.Type == key {
			return param, true
		}
	}
	return wire.KeyValuePair{}, false
}

// GenericMessage represents other message types that don't have specific structs yet.
type GenericMessage struct {
	Method_       string
	RequestID_    uint64
	TrackAlias    uint64
	Namespace     []string
	Track         string
	Authorization string
	NewSessionURI string
	ErrorCode     uint64
	ReasonPhrase  string
}

// Method returns the message type.
func (m *GenericMessage) Method() string {
	return m.Method_
}

// RequestID returns the request ID.
func (m *GenericMessage) RequestID() uint64 {
	return m.RequestID_
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
	Handle(ResponseWriter, Message)
}

// HandlerFunc is a type that implements Handler.
type HandlerFunc func(ResponseWriter, Message)

// Handle implements Handler.
func (f HandlerFunc) Handle(rw ResponseWriter, r Message) {
	f(rw, r)
}
