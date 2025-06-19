package moqtransport

type SubscribeResponseWriter struct {
	id         uint64
	trackAlias uint64
	session    *Session
	localTrack *localTrack
	handled    bool
}

// Accept accepts the subscription with default options for backward compatibility.
func (w *SubscribeResponseWriter) Accept() error {
	return w.AcceptWithOptions(DefaultSubscribeOkOptions())
}

// AcceptWithOptions accepts the subscription with custom response options.
func (w *SubscribeResponseWriter) AcceptWithOptions(opts *SubscribeOkOptions) error {
	w.handled = true
	if err := w.session.acceptSubscriptionWithOptions(w.id, opts); err != nil {
		return err
	}
	return nil
}

func (w *SubscribeResponseWriter) Reject(code uint64, reason string) error {
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
