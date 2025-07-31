package moqtransport

import "time"

type SubscribeResponseWriter struct {
	id         uint64
	trackAlias uint64
	session    *Session
	localTrack *localTrack
	handled    bool
}

// SubscribeOKOption is a functional option for configuring SUBSCRIBE_OK responses.
type SubscribeOKOption func(*SubscribeOkOptions)

// WithExpires sets the subscription expiration duration.
// A duration of 0 means the subscription never expires (default).
func WithExpires(expires time.Duration) SubscribeOKOption {
	return func(opts *SubscribeOkOptions) {
		opts.Expires = expires
	}
}

// WithGroupOrder sets the group order for the subscription.
// Default is GroupOrderAscending.
func WithGroupOrder(groupOrder GroupOrder) SubscribeOKOption {
	return func(opts *SubscribeOkOptions) {
		opts.GroupOrder = groupOrder
	}
}

// WithLargestLocation sets the largest available location for the track.
// When set, ContentExists is automatically set to true.
// When nil, ContentExists is automatically set to false.
func WithLargestLocation(location *Location) SubscribeOKOption {
	return func(opts *SubscribeOkOptions) {
		opts.LargestLocation = location
		opts.ContentExists = location != nil
	}
}

// WithParameters sets additional key-value parameters for the response.
func WithParameters(parameters KVPList) SubscribeOKOption {
	return func(opts *SubscribeOkOptions) {
		opts.Parameters = parameters
	}
}

// Accept accepts the subscription with the given options.
//
// Default behavior when no options are provided:
//   - Expires: 0 (never expires)
//   - GroupOrder: GroupOrderAscending
//   - ContentExists: false (no content available)
//   - LargestLocation: nil
//   - Parameters: empty
//
// Use WithLargestLocation to indicate content exists and provide the largest location.
// ContentExists is automatically set based on whether LargestLocation is provided.
func (w *SubscribeResponseWriter) Accept(options ...SubscribeOKOption) error {
	w.handled = true

	// Set default values
	opts := &SubscribeOkOptions{
		Expires:         0,
		GroupOrder:      GroupOrderAscending,
		ContentExists:   false,
		LargestLocation: nil,
		Parameters:      KVPList{},
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	if err := w.session.acceptSubscriptionWithOptions(w.id, opts); err != nil {
		return err
	}
	return nil
}

func (w *SubscribeResponseWriter) Reject(code ErrorCodeSubscribe, reason string) error {
	w.handled = true
	return w.session.rejectSubscription(w.id, code, reason)
}

func (w *SubscribeResponseWriter) SendDatagram(o Object) error {
	return w.localTrack.sendDatagram(o)
}

func (w *SubscribeResponseWriter) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	return w.localTrack.openSubgroup(groupID, subgroupID, priority)
}

func (w *SubscribeResponseWriter) CloseWithError(code uint64, reason string) error {
	return w.localTrack.close(code, reason)
}
