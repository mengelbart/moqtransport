package moqtransport

import "time"

const (
	SubscribeStatusUnsubscribed      = 0x00
	SubscribeStatusInternalError     = 0x01
	SubscribeStatusUnauthorized      = 0x02
	SubscribeStatusTrackEnded        = 0x03
	SubscribeStatusSubscriptionEnded = 0x04
	SubscribeStatusGoingAway         = 0x05
	SubscribeStatusExpired           = 0x06
)

type subscriptionResponse struct {
	err   error
	track *RemoteTrack
}

type Subscription struct {
	ID            uint64
	TrackAlias    uint64
	Namespace     []string
	Trackname     string
	Authorization string
	Expires       time.Duration
	GroupOrder    uint8
	ContentExists bool

	remoteTrack *RemoteTrack

	response chan subscriptionResponse
}
