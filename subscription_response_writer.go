package moqtransport

type subscriptionResponseWriter struct {
	id         uint64
	trackAlias uint64
	session    *Session
	localTrack *localTrack
	handled    bool
}

func (w *subscriptionResponseWriter) Accept() error {
	w.handled = true
	if err := w.session.acceptSubscription(w.id, w.localTrack); err != nil {
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
