package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

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

// SubscribeOption is a functional option for configuring Subscribe requests.
type SubscribeOption func(*SubscribeOptions)

// WithSubscriberPriority sets the delivery priority for the subscription.
// Priority range is 0-255, with lower values indicating higher priority (0 is highest).
// Default is 128.
func WithSubscriberPriority(priority uint8) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.SubscriberPriority = priority
	}
}

// WithSubscribeGroupOrder sets the group ordering preference for the subscription.
// Default is GroupOrderAscending.
func WithSubscribeGroupOrder(groupOrder GroupOrder) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.GroupOrder = groupOrder
	}
}

// WithForward sets the forward preference for the subscription.
// When true, indicates forward preference. Default is true.
func WithForward(forward bool) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.Forward = forward
	}
}

// WithFilterType sets the subscription filter type.
// Default is FilterTypeLatestObject.
func WithFilterType(filterType FilterType) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.FilterType = filterType
	}
}

// WithStartLocation sets the start position for absolute filters.
// Default is Location{Group: 0, Object: 0}.
func WithStartLocation(location Location) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.StartLocation = location
	}
}

// WithEndGroup sets the end group for range filters.
// Default is 0.
func WithEndGroup(endGroup uint64) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.EndGroup = endGroup
	}
}

// WithAuthorizationToken sets the authorization token for the subscription.
// This is a convenience method that adds the authorization token to parameters.
func WithAuthorizationToken(token string) SubscribeOption {
	return func(opts *SubscribeOptions) {
		if len(token) > 0 {
			// Replace existing auth token or add new one
			for i, param := range opts.Parameters {
				if param.Type == wire.AuthorizationTokenParameterKey {
					opts.Parameters[i].ValueBytes = []byte(token)
					return
				}
			}
			// Add new auth token
			opts.Parameters = append(opts.Parameters, KeyValuePair{
				Type:       wire.AuthorizationTokenParameterKey,
				ValueBytes: []byte(token),
			})
		}
	}
}

// WithSubscribeParameters sets additional key-value parameters for the subscription.
// This replaces any existing parameters.
func WithSubscribeParameters(parameters KVPList) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.Parameters = parameters
	}
}

// SubscribeUpdateOption is a functional option for configuring SUBSCRIBE_UPDATE requests.
type SubscribeUpdateOption func(*SubscribeUpdateOptions)

// WithUpdateStartLocation sets the new start position for the subscription update.
// Default is Location{Group: 0, Object: 0}. Note, should not decrease compared
// to the previous start location.
func WithUpdateStartLocation(location Location) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.StartLocation = location
	}
}

// WithUpdateEndGroup sets the new end group for the subscription update.
// EndGroup = 0 means open-ended (no end group limit). Default is 0.
func WithUpdateEndGroup(endGroup uint64) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.EndGroup = endGroup
	}
}

// WithUpdateSubscriberPriority sets the new delivery priority for the subscription update.
// Priority range is 0-255, with lower values indicating higher priority (0 is highest).
// Default is 128.
func WithUpdateSubscriberPriority(priority uint8) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.SubscriberPriority = priority
	}
}

// WithUpdateForward sets the new forward preference for the subscription update.
// When true, indicates forward preference. Default is true.
func WithUpdateForward(forward bool) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.Forward = forward
	}
}

// WithUpdateParameters sets additional key-value parameters for the subscription update.
// This replaces any existing parameters.
func WithUpdateParameters(parameters KVPList) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.Parameters = parameters
	}
}
