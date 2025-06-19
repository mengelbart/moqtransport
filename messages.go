package moqtransport

import (
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
)

// Location represents a MoQ object location consisting of Group and Object IDs.
// This is used to specify positions within the media stream for subscriptions
// and other operations that require location information.
type Location struct {
	Group  uint64 // Group ID (typically corresponds to GOP/segment)
	Object uint64 // Object ID within the group
}

// toWireLocation converts a public Location to an internal wire.Location
func (l *Location) toWireLocation() wire.Location {
	return wire.Location{
		Group:  l.Group,
		Object: l.Object,
	}
}

// FilterType represents the subscription filter type used in SUBSCRIBE messages.
type FilterType uint64

const (
	// FilterTypeLatestObject starts from the latest available object.
	FilterTypeLatestObject FilterType = 0x02

	// FilterTypeNextGroupStart starts from the beginning of the next group.
	FilterTypeNextGroupStart FilterType = 0x01

	// FilterTypeAbsoluteStart starts from a specific absolute position.
	FilterTypeAbsoluteStart FilterType = 0x03

	// FilterTypeAbsoluteRange subscribes to a specific range of groups/objects.
	FilterTypeAbsoluteRange FilterType = 0x04
)

// toWireFilterType converts a public FilterType to an internal wire.FilterType
func (f FilterType) toWireFilterType() wire.FilterType {
	return wire.FilterType(f)
}

// KeyValuePair represents a key-value parameter pair.
type KeyValuePair struct {
	Type        uint64
	ValueVarInt uint64
	ValueBytes  []byte
}

// toWireKVP converts a public KeyValuePair to an internal wire.KeyValuePair
func (kvp *KeyValuePair) toWireKVP() wire.KeyValuePair {
	return wire.KeyValuePair{
		Type:        kvp.Type,
		ValueVarInt: kvp.ValueVarInt,
		ValueBytes:  kvp.ValueBytes,
	}
}

// KVPList represents a list of key-value parameters.
type KVPList []KeyValuePair

// toWireKVPList converts a public KVPList to an internal wire.KVPList
func (kvpl KVPList) toWireKVPList() wire.KVPList {
	result := make(wire.KVPList, len(kvpl))
	for i, kvp := range kvpl {
		result[i] = kvp.toWireKVP()
	}
	return result
}

// fromWireLocation converts an internal wire.Location to a public Location
func fromWireLocation(wl wire.Location) Location {
	return Location{
		Group:  wl.Group,
		Object: wl.Object,
	}
}

// fromWireFilterType converts an internal wire.FilterType to a public FilterType
func fromWireFilterType(wf wire.FilterType) FilterType {
	return FilterType(wf)
}

// fromWireKVP converts an internal wire.KeyValuePair to a public KeyValuePair
func fromWireKVP(wkvp wire.KeyValuePair) KeyValuePair {
	return KeyValuePair{
		Type:        wkvp.Type,
		ValueVarInt: wkvp.ValueVarInt,
		ValueBytes:  wkvp.ValueBytes,
	}
}

// fromWireKVPList converts an internal wire.KVPList to a public KVPList
func fromWireKVPList(wkvpl wire.KVPList) KVPList {
	result := make(KVPList, len(wkvpl))
	for i, wkvp := range wkvpl {
		result[i] = fromWireKVP(wkvp)
	}
	return result
}

// SubscribeOptions contains options for subscribing to a track with full control
// over all subscribe message parameters.
type SubscribeOptions struct {
	// SubscriberPriority indicates the delivery priority (0-255, higher is more important)
	SubscriberPriority uint8

	// GroupOrder indicates group ordering preference:
	// 0 = None (no specific ordering), 1 = Ascending, 2 = Descending
	GroupOrder uint8

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
		GroupOrder:         1,
		Forward:            true,
		FilterType:         FilterTypeNextGroupStart,
		StartLocation:      Location{0, 0},
		EndGroup:           0,
		Parameters:         KVPList{},
	}
}

// SubscribeOkOptions contains options for customizing subscription acceptance responses.
type SubscribeOkOptions struct {
	// Expires specifies how long the subscription is valid
	Expires time.Duration

	// GroupOrder specifies the actual group order that will be used
	GroupOrder uint8

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
		GroupOrder:      1,
		ContentExists:   true,
		LargestLocation: nil,
		Parameters:      KVPList{},
	}
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
	GroupOrder         uint8      // Group ordering preference: 0=None, 1=Ascending, 2=Descending
	Forward            uint8      // Forward preference: 0=No, 1=Yes
	FilterType         FilterType // Subscription filter type
	StartLocation      *Location  // Start position for absolute filters
	EndGroup           *uint64    // End group for range filters
	Parameters         KVPList    // Full parameter list from the subscribe message
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
	RequestID uint64

	// Subscribe update specific fields
	StartLocation      Location  // New start position for the subscription
	EndGroup           uint64    // New end group for the subscription
	SubscriberPriority uint8     // Updated delivery priority (0-255, higher is more important)
	Forward            uint8     // Updated forward preference: 0=No, 1=Yes
	Parameters         KVPList   // Updated parameter list
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

// GetMaxCacheDuration extracts the max cache duration parameter if present.
func (m *SubscribeUpdateMessage) GetMaxCacheDuration() (time.Duration, bool) {
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
func (m *SubscribeUpdateMessage) GetParameter(key uint64) (KeyValuePair, bool) {
	for _, param := range m.Parameters {
		if param.Type == key {
			return param, true
		}
	}
	return KeyValuePair{}, false
}
