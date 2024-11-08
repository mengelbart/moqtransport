package moqtransport

type SubscriptionHandler interface {
	HandleSubscription(*Transport, Subscription, SubscriptionResponseWriter)
}

type SubscriptionHandlerFunc func(*Transport, Subscription, SubscriptionResponseWriter)

func (f SubscriptionHandlerFunc) HandleSubscription(se *Transport, su Subscription, srw SubscriptionResponseWriter) {
	f(se, su, srw)
}
