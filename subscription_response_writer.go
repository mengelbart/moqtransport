package moqtransport

import "errors"

var errSubscriptionNotAccepted = errors.New("publish before subscription accepted")

type subscriptionResponseWriter struct {
	id         uint64
	trackAlias uint64
	transport  *Transport
	localTrack *LocalTrack
}

func (w *subscriptionResponseWriter) Accept() error {
	w.localTrack = newLocalTrack(w.transport.conn, w.id, w.trackAlias)
	if err := w.transport.acceptSubscription(w.id, w.localTrack); err != nil {
		return err
	}
	return nil
}

func (w *subscriptionResponseWriter) Reject(code uint64, reason string) error {
	return w.transport.rejectSubscription(w.id, code, reason)
}

func (w *subscriptionResponseWriter) SendDatagram(o Object) error {
	if w.localTrack == nil {
		return errSubscriptionNotAccepted
	}
	return w.localTrack.SendDatagram(o)
}

func (w *subscriptionResponseWriter) OpenSubgroup(groupID uint64, priority uint8) (*Subgroup, error) {
	if w.localTrack == nil {
		return nil, errSubscriptionNotAccepted
	}
	return w.localTrack.OpenSubgroup(groupID, priority)
}

func (w *subscriptionResponseWriter) Close() error {
	if w.localTrack == nil {
		return errSubscriptionNotAccepted
	}
	return w.localTrack.Close()
}
