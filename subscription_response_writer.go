package moqtransport

type SubscriptionResponseWriter interface {
	Accept() (*Publisher, error)
	Reject(code uint64, reason string) error
}

type defaultSubscriptionResponseWriter struct {
	subscription Subscription
	transport    *Transport
}

func (w *defaultSubscriptionResponseWriter) Accept() (*Publisher, error) {
	w.subscription.publisher = newPublisher(w.transport.conn, w.subscription.ID, w.subscription.TrackAlias)
	if err := w.transport.acceptSubscription(w.subscription); err != nil {
		return nil, err
	}
	return w.subscription.publisher, nil
}

func (w *defaultSubscriptionResponseWriter) Reject(code uint64, reason string) error {
	return w.transport.rejectSubscription(w.subscription, code, reason)
}
