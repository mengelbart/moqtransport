package moqtransport

type SubscriptionResponseWriter interface {
	Accept(*LocalTrack)
	Reject(code uint64, reason string)
}

type defaultSubscriptionResponseWriter struct {
	subscription *Subscription
	session      *Session
}

func (w *defaultSubscriptionResponseWriter) Accept(t *LocalTrack) {
	w.session.subscribeToLocalTrack(w.subscription, t)
}

func (w *defaultSubscriptionResponseWriter) Reject(code uint64, reason string) {
	w.session.rejectSubscription(w.subscription, code, reason)
}

type SubscriptionHandler interface {
	HandleSubscription(*Session, *Subscription, SubscriptionResponseWriter)
}

type SubscriptionHandlerFunc func(*Session, *Subscription, SubscriptionResponseWriter)

func (f SubscriptionHandlerFunc) HandleSubscription(se *Session, su *Subscription, srw SubscriptionResponseWriter) {
	f(se, su, srw)
}
