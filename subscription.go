package moqtransport

const (
	SubscribeStatusUnsubscribed      = 0x00
	SubscribeStatusInternalError     = 0x01
	SubscribeStatusUnauthorized      = 0x02
	SubscribeStatusTrackEnded        = 0x03
	SubscribeStatusSubscriptionEnded = 0x04
	SubscribeStatusGoingAway         = 0x05
	SubscribeStatusExpired           = 0x06
)

type Subscription struct {
	ID            uint64
	TrackAlias    uint64
	Namespace     string
	TrackName     string
	Authorization string
}

type SubscriptionResponseWriter interface {
	Accept(*LocalTrack)
	Reject(code uint64, reason string)
}

type SubscriptionHandler interface {
	HandleSubscription(*Session, *Subscription, SubscriptionResponseWriter)
}

type SubscriptionHandlerFunc func(*Session, *Subscription, SubscriptionResponseWriter)

func (f SubscriptionHandlerFunc) HandleSubscription(se *Session, su *Subscription, srw SubscriptionResponseWriter) {
	f(se, su, srw)
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
