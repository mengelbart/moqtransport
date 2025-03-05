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

const (
	maxVarint uint64 = 1<<62 - 1
)
