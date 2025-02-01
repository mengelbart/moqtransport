package moqtransport

import "errors"

var errSubscriptionNotAccepted = errors.New("publish before subscription accepted")

type subscriptionResponseWriter struct {
	subscription *subscription
	transport    *Transport
}

func (w *subscriptionResponseWriter) Accept() error {
	w.subscription.localTrack = newLocalTrack(w.transport.conn, w.subscription.ID, w.subscription.TrackAlias)
	if err := w.transport.acceptSubscription(w.subscription); err != nil {
		return err
	}
	return nil
}

func (w *subscriptionResponseWriter) Reject(code uint64, reason string) error {
	return w.transport.rejectSubscription(w.subscription, code, reason)
}

func (w *subscriptionResponseWriter) SendDatagram(o Object) error {
	if w.subscription.localTrack == nil {
		return errSubscriptionNotAccepted
	}
	return w.subscription.localTrack.SendDatagram(o)
}

func (w *subscriptionResponseWriter) OpenSubgroup(groupID uint64, priority uint8) (*Subgroup, error) {
	if w.subscription.localTrack == nil {
		return nil, errSubscriptionNotAccepted
	}
	return w.subscription.localTrack.OpenSubgroup(groupID, priority)
}

func (w *subscriptionResponseWriter) Close() error {
	return w.subscription.localTrack.Close()
}
