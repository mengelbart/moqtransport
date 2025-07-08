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

// DefaultSubscribeOptions returns a reasonable default set of options for subscriptions.
func DefaultSubscribeOptions() *SubscribeOptions {
	return &SubscribeOptions{
		SubscriberPriority: 128,
		GroupOrder:         GroupOrderAscending,
		Forward:            true,
		FilterType:         FilterTypeLatestObject,
		StartLocation:      Location{Group: 0, Object: 0},
		EndGroup:           0,
		Parameters:         KVPList{},
	}
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

// DefaultSubscribeOkOptions returns a default set of options for SubscribeOk responses.
func DefaultSubscribeOkOptions() *SubscribeOkOptions {
	return &SubscribeOkOptions{
		Expires:         0,
		GroupOrder:      GroupOrderAscending,
		ContentExists:   true,
		LargestLocation: &Location{Group: 0, Object: 0},
		Parameters:      KVPList{},
	}
}

// SubscriptionInfo contains all information received from a SUBSCRIBE_OK response.
// This provides clients with complete metadata about the accepted subscription.
type SubscriptionInfo struct {
	// Expires specifies how long the subscription is valid in milliseconds.
	// A value of 0 indicates that the subscription does not expire or expires at an unknown time.
	// Expires is advisory and a subscription can end prior to the expiry time or last longer.
	Expires time.Duration

	// GroupOrder indicates the subscription will be delivered in a specific order by group.
	// See GroupOrder constants for valid values.
	GroupOrder GroupOrder

	// ContentExists indicates whether content has been published on this track.
	// true if an object has been published, false if not.
	ContentExists bool

	// LargestLocation contains the location of the largest object available for this track.
	// This field is only present if ContentExists is true.
	// Can be used for optimal track switching by calculating switching boundaries.
	LargestLocation *Location

	// Parameters contains the key-value parameters from the SUBSCRIBE_OK response.
	// These may include publisher-specific metadata or delivery preferences.
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

// DefaultSubscribeUpdateOptions returns a reasonable default set of options for subscription updates.
func DefaultSubscribeUpdateOptions() *SubscribeUpdateOptions {
	return &SubscribeUpdateOptions{
		StartLocation: Location{
			Group:  0,
			Object: 0,
		},
		EndGroup:           0,
		SubscriberPriority: 128,
		Forward:            true,
		Parameters:         KVPList{},
	}
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
