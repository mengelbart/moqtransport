package moqtransport

import (
	"errors"
	"fmt"
)

// GroupOrder specifies the order in which groups should be delivered
type GroupOrder uint8

const (
	// GroupOrderAscending indicates ascending order (oldest first)
	GroupOrderAscending GroupOrder = 0x00
	// GroupOrderDescending indicates descending order (newest first)
	GroupOrderDescending GroupOrder = 0x01
	// GroupOrderNone indicates no preference
	GroupOrderNone GroupOrder = 0x02
)

// FilterType specifies the type of subscription filter
type FilterType uint8

const (
	// FilterTypeNone indicates no filtering
	FilterTypeNone FilterType = 0x00
	// FilterTypeLatestGroup subscribes from the latest group
	FilterTypeLatestGroup FilterType = 0x01
	// FilterTypeLatestObject subscribes from the latest object
	FilterTypeLatestObject FilterType = 0x02
	// FilterTypeAbsoluteStart subscribes from a specific start point
	FilterTypeAbsoluteStart FilterType = 0x03
	// FilterTypeAbsoluteRange subscribes to a specific range
	FilterTypeAbsoluteRange FilterType = 0x04
)

// FullSequence represents a complete group and object sequence pair
type FullSequence struct {
	// Group is the group sequence number
	Group uint64
	// Object is the object sequence number within the group
	Object uint64
}

// Parameters represents additional key-value parameters for messages
type Parameters map[string]interface{}

// SubscribeError represents error codes for SUBSCRIBE_ERROR messages
type SubscribeError uint64

// Common subscribe error codes
const (
	SubscribeErrorInternalError     SubscribeError = 0x00
	SubscribeErrorUnauthorized      SubscribeError = 0x01
	SubscribeErrorTrackDoesNotExist SubscribeError = 0x02
	SubscribeErrorGeneric           SubscribeError = 0x03
)

// AnnounceError represents error codes for ANNOUNCE_ERROR messages
type AnnounceError uint64

// Common announce error codes
const (
	AnnounceErrorInternalError      AnnounceError = 0x00
	AnnounceErrorUnauthorized       AnnounceError = 0x01
	AnnounceErrorDuplicateNamespace AnnounceError = 0x02
	AnnounceErrorGeneric            AnnounceError = 0x03
)

// TypedMessage is the interface implemented by all typed MoQ control messages.
// Each message type provides access to its specific fields while
// maintaining a common interface for message handling.
type TypedMessage interface {
	// Type returns the message type as a string (e.g., "subscribe", "announce")
	Type() string
}

// SubscribeMessage represents a SUBSCRIBE message with all its parameters.
// This message is sent by a subscriber to request a track from a publisher.
type SubscribeMessage struct {
	// RequestID is the unique identifier for this subscription request
	RequestID uint64

	// TrackAlias is a session-specific identifier for this track subscription
	TrackAlias uint64

	// TrackNamespace identifies the namespace of the track
	TrackNamespace string

	// TrackName identifies the specific track within the namespace
	TrackName string

	// SubscriberPriority indicates the priority of this subscription (0-255)
	// Higher values indicate higher priority
	SubscriberPriority uint8

	// GroupOrder indicates the order in which groups should be delivered
	GroupOrder GroupOrder

	// FilterType indicates the type of filtering requested
	FilterType FilterType

	// StartGroup indicates the starting group for the subscription (inclusive)
	// Only valid for certain filter types
	StartGroup *FullSequence

	// StartObject indicates the starting object for the subscription (inclusive)
	// Only valid for certain filter types
	StartObject *FullSequence

	// EndGroup indicates the ending group for the subscription (inclusive)
	// Only valid for certain filter types
	EndGroup *FullSequence

	// EndObject indicates the ending object for the subscription (inclusive)
	// Only valid for certain filter types
	EndObject *FullSequence

	// Parameters contains additional parameters for the subscription
	Parameters Parameters
}

func (m *SubscribeMessage) Type() string { return MessageSubscribe }

// SubscribeOkMessage represents a SUBSCRIBE_OK response message.
// This is sent by a publisher to accept a subscription request.
type SubscribeOkMessage struct {
	// TrackAlias echoes the track alias from the subscription request
	TrackAlias uint64

	// Expires indicates when the subscription expires (milliseconds)
	// 0 means no expiration
	Expires uint64

	// GroupOrder indicates the order in which groups will be delivered
	// If not specified, uses the subscriber's requested order
	GroupOrder *GroupOrder

	// ContentExists indicates whether the track has content available
	ContentExists bool

	// FinalGroup indicates the final group ID if the track has ended
	FinalGroup *uint64

	// FinalObject indicates the final object ID in the final group
	FinalObject *uint64

	// Parameters contains additional parameters for the response
	Parameters Parameters
}

func (m *SubscribeOkMessage) Type() string { return "subscribe_ok" }

// SubscribeErrorMessage represents a SUBSCRIBE_ERROR response message.
// This is sent by a publisher to reject a subscription request.
type SubscribeErrorMessage struct {
	// TrackAlias echoes the track alias from the subscription request
	TrackAlias uint64

	// Code indicates the error code
	Code SubscribeError

	// ReasonPhrase provides a human-readable error description
	ReasonPhrase string
}

func (m *SubscribeErrorMessage) Type() string { return "subscribe_error" }

// SubscribeUpdateMessage represents a SUBSCRIBE_UPDATE message.
// This is sent by a subscriber to update subscription parameters.
type SubscribeUpdateMessage struct {
	// TrackAlias identifies the subscription to update
	TrackAlias uint64

	// SubscriberPriority is the new priority value
	SubscriberPriority uint8

	// Parameters contains additional parameters for the update
	Parameters Parameters
}

func (m *SubscribeUpdateMessage) Type() string { return MessageSubscribeUpdate }

// AnnounceMessage represents an ANNOUNCE message.
// This is sent by a publisher to announce availability of a namespace.
type AnnounceMessage struct {
	// TrackNamespace is the namespace being announced
	TrackNamespace string

	// Parameters contains additional parameters for the announcement
	Parameters Parameters
}

func (m *AnnounceMessage) Type() string { return MessageAnnounce }

// AnnounceOkMessage represents an ANNOUNCE_OK response message.
// This is sent to accept an announcement.
type AnnounceOkMessage struct {
	// TrackNamespace echoes the namespace from the announcement
	TrackNamespace string
}

func (m *AnnounceOkMessage) Type() string { return "announce_ok" }

// AnnounceErrorMessage represents an ANNOUNCE_ERROR response message.
// This is sent to reject an announcement.
type AnnounceErrorMessage struct {
	// TrackNamespace echoes the namespace from the announcement
	TrackNamespace string

	// Code indicates the error code
	Code AnnounceError

	// ReasonPhrase provides a human-readable error description
	ReasonPhrase string
}

func (m *AnnounceErrorMessage) Type() string { return "announce_error" }

// UnsubscribeMessage represents an UNSUBSCRIBE message.
// This is sent by a subscriber to cancel a subscription.
type UnsubscribeMessage struct {
	// TrackAlias identifies the subscription to cancel
	TrackAlias uint64
}

func (m *UnsubscribeMessage) Type() string { return MessageUnsubscribe }

// Message builder functions with validation

// SubscribeOption is a functional option for building SubscribeMessage
type SubscribeOption func(*SubscribeMessage) error

// NewSubscribeMessage creates a new SubscribeMessage with the provided options.
// It validates that required fields are set.
func NewSubscribeMessage(opts ...SubscribeOption) (*SubscribeMessage, error) {
	msg := &SubscribeMessage{
		GroupOrder:         GroupOrderAscending,  // default
		FilterType:         FilterTypeLatestGroup, // default
		SubscriberPriority: 128,                  // default mid-priority
		Parameters:         make(Parameters),
	}

	for _, opt := range opts {
		if err := opt(msg); err != nil {
			return nil, err
		}
	}

	// Validate required fields
	if msg.TrackNamespace == "" {
		return nil, errors.New("track namespace is required")
	}
	if msg.TrackName == "" {
		return nil, errors.New("track name is required")
	}

	return msg, nil
}

// WithTrackAlias sets the track alias for the subscription
func WithTrackAlias(alias uint64) SubscribeOption {
	return func(m *SubscribeMessage) error {
		m.TrackAlias = alias
		return nil
	}
}

// WithTrackNamespace sets the track namespace
func WithTrackNamespace(namespace string) SubscribeOption {
	return func(m *SubscribeMessage) error {
		if namespace == "" {
			return errors.New("track namespace cannot be empty")
		}
		m.TrackNamespace = namespace
		return nil
	}
}

// WithTrackName sets the track name
func WithTrackName(name string) SubscribeOption {
	return func(m *SubscribeMessage) error {
		if name == "" {
			return errors.New("track name cannot be empty")
		}
		m.TrackName = name
		return nil
	}
}

// WithSubscriberPriority sets the subscriber priority
func WithSubscriberPriority(priority uint8) SubscribeOption {
	return func(m *SubscribeMessage) error {
		m.SubscriberPriority = priority
		return nil
	}
}

// WithGroupOrder sets the group order preference
func WithGroupOrder(order GroupOrder) SubscribeOption {
	return func(m *SubscribeMessage) error {
		m.GroupOrder = order
		return nil
	}
}

// WithFilterType sets the filter type and validates range parameters
func WithFilterType(filterType FilterType) SubscribeOption {
	return func(m *SubscribeMessage) error {
		m.FilterType = filterType
		return nil
	}
}

// WithRange sets the start and end range for filtered subscriptions
func WithRange(startGroup, startObject, endGroup, endObject *FullSequence) SubscribeOption {
	return func(m *SubscribeMessage) error {
		// Validate that if start is set, it's valid
		if startGroup != nil && startObject == nil {
			return errors.New("start object must be specified with start group")
		}
		if endGroup != nil && endObject == nil {
			return errors.New("end object must be specified with end group")
		}

		m.StartGroup = startGroup
		m.StartObject = startObject
		m.EndGroup = endGroup
		m.EndObject = endObject
		return nil
	}
}

// WithParameters adds parameters to the subscription
func WithParameters(params Parameters) SubscribeOption {
	return func(m *SubscribeMessage) error {
		if m.Parameters == nil {
			m.Parameters = make(Parameters)
		}
		for k, v := range params {
			m.Parameters[k] = v
		}
		return nil
	}
}

// SubscribeOkOption is a functional option for building SubscribeOkMessage
type SubscribeOkOption func(*SubscribeOkMessage) error

// NewSubscribeOkMessage creates a new SubscribeOkMessage for responding to a subscription
func NewSubscribeOkMessage(trackAlias uint64, opts ...SubscribeOkOption) (*SubscribeOkMessage, error) {
	msg := &SubscribeOkMessage{
		TrackAlias:    trackAlias,
		ContentExists: true, // default to content existing
		Parameters:    make(Parameters),
	}

	for _, opt := range opts {
		if err := opt(msg); err != nil {
			return nil, err
		}
	}

	return msg, nil
}

// WithExpires sets the expiration time for the subscription
func WithExpires(expires uint64) SubscribeOkOption {
	return func(m *SubscribeOkMessage) error {
		m.Expires = expires
		return nil
	}
}

// WithGroupOrderResponse sets the group order that will be used
func WithGroupOrderResponse(order GroupOrder) SubscribeOkOption {
	return func(m *SubscribeOkMessage) error {
		m.GroupOrder = &order
		return nil
	}
}

// WithContentExists sets whether content exists for the track
func WithContentExists(exists bool) SubscribeOkOption {
	return func(m *SubscribeOkMessage) error {
		m.ContentExists = exists
		return nil
	}
}

// WithFinalGroup sets the final group and object IDs for a completed track
func WithFinalGroup(group, object uint64) SubscribeOkOption {
	return func(m *SubscribeOkMessage) error {
		m.FinalGroup = &group
		m.FinalObject = &object
		return nil
	}
}

// WithOkParameters adds parameters to the response
func WithOkParameters(params Parameters) SubscribeOkOption {
	return func(m *SubscribeOkMessage) error {
		if m.Parameters == nil {
			m.Parameters = make(Parameters)
		}
		for k, v := range params {
			m.Parameters[k] = v
		}
		return nil
	}
}

// Helper to convert old Message to new typed messages (temporary during migration)
// NOTE: This is limited by the fields available in the current Message struct.
// Some fields like GroupOrder, FilterType, etc. are not yet passed through from
// the wire messages to the Message struct in session.go
func messageToTyped(msg *Message) (TypedMessage, error) {
	switch msg.Method {
	case MessageSubscribe:
		// Convert namespace slice to string (typically single element)
		namespace := ""
		if len(msg.Namespace) > 0 {
			namespace = msg.Namespace[0] // For now, use first element
		}
		
		// Extract authorization from parameters if available
		params := make(Parameters)
		if msg.Authorization != "" {
			params["authorization"] = msg.Authorization
		}
		
		return &SubscribeMessage{
			RequestID:          msg.RequestID,
			TrackAlias:         msg.TrackAlias,
			TrackNamespace:     namespace,
			TrackName:          msg.Track,
			SubscriberPriority: msg.SubscriberPriority,
			// TODO: These fields are not yet available in Message struct:
			// GroupOrder, FilterType, StartGroup, StartObject, EndGroup, EndObject
			GroupOrder: GroupOrderAscending, // default
			FilterType: FilterTypeLatestGroup, // default
			Parameters: params,
		}, nil

	case MessageSubscribeUpdate:
		return &SubscribeUpdateMessage{
			TrackAlias:         msg.TrackAlias,
			SubscriberPriority: msg.SubscriberPriority,
			Parameters:         make(Parameters), // TODO: Parameters not in Message yet
		}, nil

	case MessageAnnounce:
		// Convert namespace slice to string
		namespace := ""
		if len(msg.Namespace) > 0 {
			namespace = msg.Namespace[0]
		}
		
		return &AnnounceMessage{
			TrackNamespace: namespace,
			Parameters:     make(Parameters), // TODO: Parameters not in Message yet
		}, nil

	case MessageUnsubscribe:
		return &UnsubscribeMessage{
			TrackAlias: msg.TrackAlias,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported message type for conversion: %s", msg.Method)
	}
}