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
	TrackStatusInProgress   = 0x00
	TrackStatusDoesNotExist = 0x01
	TrackStatusNotYetBegun  = 0x02
	TrackStatusFinished     = 0x03
	TrackStatusUnavailable  = 0x04
)
