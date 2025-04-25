package moqtransport

type subscriptionResponseWriter struct {
	id         uint64
	trackAlias uint64
	session    *Session
	localTrack *localTrack
	handled    bool
}

// SubscriptionResponseWriter provides extra methods for handling subscription requests.
type SubscriptionResponseWriter interface {
	SubscribeID() uint64
	TrackAlias() uint64
	ResponseWriter
}

// Session returns the session associated with this response writer.
func (w *subscriptionResponseWriter) Session() *Session {
	return w.session
}

// SubscribeID returns the subscribeID of the subscription request.
func (w *subscriptionResponseWriter) SubscribeID() uint64 {
	return w.id
}

// TrackAlias returns the track alias of the subscription request.
func (w *subscriptionResponseWriter) TrackAlias() uint64 {
	return w.trackAlias
}

func (w *subscriptionResponseWriter) Accept() error {
	w.handled = true
	if err := w.session.acceptSubscription(w.id); err != nil {
		return err
	}
	return nil
}

func (w *subscriptionResponseWriter) Reject(code uint64, reason string) error {
	w.handled = true
	return w.session.rejectSubscription(w.id, code, reason)
}

func (w *subscriptionResponseWriter) SendDatagram(o Object) error {
	return w.localTrack.sendDatagram(o)
}

func (w *subscriptionResponseWriter) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	return w.localTrack.openSubgroup(groupID, subgroupID, priority)
}

func (w *subscriptionResponseWriter) CloseWithError(code uint64, reason string) error {
	return w.localTrack.close(code, reason)
}
