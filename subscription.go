package moqtransport

import (
	"time"
)

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

type subscription struct {
	id            uint64
	trackAlias    uint64
	namespace     []string
	trackname     string
	authorization string
	expires       time.Duration
	groupOrder    uint8
	contentExists bool

	remoteTrack *RemoteTrack
	localTrack  *localTrack

	isFetch bool

	response chan subscriptionResponse
}
