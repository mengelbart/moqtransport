package moqtransport

import (
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type Location = wire.Location

// FilterType represents the subscription filter type used in SUBSCRIBE messages.
type FilterType = wire.FilterType

const (
	// FilterTypeLatestObject starts from the latest available object.
	FilterTypeLatestObject FilterType = wire.FilterTypeLatestObject

	// FilterTypeNextGroupStart starts from the beginning of the next group.
	FilterTypeNextGroupStart FilterType = wire.FilterTypeNextGroupStart

	// FilterTypeAbsoluteStart starts from a specific absolute position.
	FilterTypeAbsoluteStart FilterType = wire.FilterTypeAbsoluteStart

	// FilterTypeAbsoluteRange subscribes to a specific range of groups/objects.
	FilterTypeAbsoluteRange FilterType = wire.FilterTypeAbsoluteRange
)

// GroupOrder represents the group delivery order preference used in SUBSCRIBE_OK messages.
type GroupOrder = wire.GroupOrder

const (
	// GroupOrderNone indicates no specific ordering preference.
	GroupOrderNone GroupOrder = wire.GroupOrderNone

	// GroupOrderAscending indicates groups should be delivered in ascending order.
	GroupOrderAscending GroupOrder = wire.GroupOrderAscending

	// GroupOrderDescending indicates groups should be delivered in descending order.
	GroupOrderDescending GroupOrder = wire.GroupOrderDescending
)

// KeyValuePair represents a key-value parameter pair.
type KeyValuePair = wire.KeyValuePair

// KVPList represents a list of key-value parameters.
type KVPList []KeyValuePair

// ToWire creates a deep copy of KVPList as wire.KVPList to prevent mutation.
func (kvpl KVPList) ToWire() wire.KVPList {
	if kvpl == nil {
		return nil
	}
	result := make(wire.KVPList, len(kvpl))
	for i, param := range kvpl {
		result[i] = wire.KeyValuePair{
			Type:        param.Type,
			ValueVarInt: param.ValueVarInt,
			ValueBytes:  append([]byte(nil), param.ValueBytes...),
		}
	}
	return result
}

// FromWire creates a deep copy of wire.KVPList as KVPList to prevent mutation.
func FromWire(wireKVP wire.KVPList) KVPList {
	if wireKVP == nil {
		return nil
	}
	result := make(KVPList, len(wireKVP))
	for i, param := range wireKVP {
		result[i] = KeyValuePair{
			Type:        param.Type,
			ValueVarInt: param.ValueVarInt,
			ValueBytes:  append([]byte(nil), param.ValueBytes...),
		}
	}
	return result
}

// GetParameter extracts a specific parameter by key from the parameter list.
func (kvpl KVPList) GetParameter(key uint64) (KeyValuePair, bool) {
	for _, param := range kvpl {
		if param.Type == key {
			return param, true
		}
	}
	return KeyValuePair{}, false
}

// GetDeliveryTimeout extracts the delivery timeout parameter if present.
// Returns the timeout duration in milliseconds and whether the parameter was found.
func (kvpl KVPList) GetDeliveryTimeout() (time.Duration, bool) {
	for _, param := range kvpl {
		if param.Type == wire.DeliveryTimeoutParameterKey {
			return time.Duration(param.ValueVarInt) * time.Millisecond, true
		}
	}
	return 0, false
}

// GetMaxCacheDuration extracts the max cache duration parameter if present.
// Returns the cache duration and whether the parameter was found.
func (kvpl KVPList) GetMaxCacheDuration() (time.Duration, bool) {
	for _, param := range kvpl {
		if param.Type == wire.MaxCacheDurationParameterKey {
			if len(param.ValueBytes) > 0 {
				// TODO: Parse duration from bytes according to specification
				// For now, return zero duration as placeholder
				return 0, true
			}
			// If no bytes, treat as varInt milliseconds
			return time.Duration(param.ValueVarInt) * time.Millisecond, true
		}
	}
	return 0, false
}

// GetAuthorizationToken extracts the authorization token parameter if present.
// Returns the token as a byte slice and whether the parameter was found.
func (kvpl KVPList) GetAuthorizationToken() ([]byte, bool) {
	for _, param := range kvpl {
		if param.Type == wire.AuthorizationTokenParameterKey {
			if len(param.ValueBytes) > 0 {
				return param.ValueBytes, true
			}
		}
	}
	return nil, false
}

// SubscribeOptions contains options for subscribing to a track with full control
// over all subscribe message parameters.
type SubscribeOptions struct {
	// SubscriberPriority indicates the delivery priority (0-255, higher is more important)
	SubscriberPriority uint8

	// GroupOrder indicates group ordering preference:
	// 0 = None (no specific ordering), 1 = Ascending, 2 = Descending
	GroupOrder GroupOrder

	// Forward indicates forward preference:
	// false = No forward preference, true Forward preference
	Forward bool // (true = 1, false = 0)

	// FilterType specifies the subscription filter type
	FilterType FilterType

	// StartLocation specifies the start position for absolute filters
	StartLocation Location

	// EndGroup specifies the end group for range filters
	EndGroup uint64

	// Parameters contains key-value parameters for the subscription
	Parameters KVPList
}

// SubscribeOkOptions contains options for customizing subscription acceptance responses.
type SubscribeOkOptions struct {
	// Expires specifies how long the subscription is valid
	Expires time.Duration

	// GroupOrder specifies the actual group order that will be used
	GroupOrder GroupOrder

	// ContentExists indicates whether content is available for this track
	ContentExists bool

	// LargestLocation specifies the largest available location if content exists
	LargestLocation *Location

	// Parameters contains response parameters
	Parameters KVPList
}

// SubscribeUpdateOptions contains options for updating an existing subscription.
type SubscribeUpdateOptions struct {
	// StartLocation specifies the new start position for the subscription
	StartLocation Location

	// EndGroup specifies the new end group for the subscription
	EndGroup uint64

	// SubscriberPriority indicates the new delivery priority (0-255, higher is more important)
	SubscriberPriority uint8

	// Forward indicates the new forward preference:
	// false = No forward preference, true = Forward preference
	Forward bool

	// Parameters contains key-value parameters for the update
	Parameters KVPList
}

// SubscribeMessage represents a SUBSCRIBE message from the peer.
type SubscribeMessage struct {
	RequestID  uint64
	TrackAlias uint64
	Namespace  []string
	Track      string

	// Authorization token should be an object, see 8.2.1.1
	Authorization string

	// Subscribe message specific fields
	SubscriberPriority uint8      // Delivery priority (0-255, higher is more important)
	GroupOrder         GroupOrder // Group ordering preference: 0=None, 1=Ascending, 2=Descending
	Forward            uint8      // Forward preference: 0=No, 1=Yes
	FilterType         FilterType // Subscription filter type
	StartLocation      *Location  // Start position for absolute filters
	EndGroup           *uint64    // End group for range filters
	Parameters         KVPList    // Full parameter list from the subscribe message
}

// SubscribeUpdateMessage represents a SUBSCRIBE_UPDATE message from the peer.
type SubscribeUpdateMessage struct {
	RequestID uint64

	// Subscribe update specific fields
	StartLocation      Location // New start position for the subscription
	EndGroup           uint64   // New end group for the subscription
	SubscriberPriority uint8    // Updated delivery priority (0-255, higher is more important)
	Forward            uint8    // Updated forward preference: 0=No, 1=Yes
	Parameters         KVPList  // Updated parameter list
}
