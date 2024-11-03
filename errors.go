package moqtransport

import "fmt"

const (
	ErrorCodeNoError                 = 0x00
	ErrorCodeInternal                = 0x01
	ErrorCodeUnauthorized            = 0x02
	ErrorCodeProtocolViolation       = 0x03
	ErrorCodeDuplicateTrackAlias     = 0x04
	ErrorCodeParameterLengthMismatch = 0x05
	ErrorTooManySubscribes           = 0x06
	ErrorCodeGoAwayTimeout           = 0x10

	// Errors not included in current draft
	ErrorCodeUnsupportedVersion = 0xff01
)

const (
	SubscribeErrorInternal          = 0x00
	SubscribeErrorInvalidRange      = 0x01
	SubscribeErrorRetryTrackAlias   = 0x02
	SubscribeErrorTrackDoesNotExist = 0x03
	SubscribeErrorUnauthorized      = 0x04
	SubscribeErrorTimeout           = 0x05
)

const (
	SubscribeDoneUnsubscribed  = 0x00
	SubscribeDoneInternalError = 0x01
	SubscribeDoneUnauthorized  = 0x02
	SubscribeDoneTrackEnded    = 0x03
	SubscribeDoneGoingAway     = 0x04
	SubscribeDoneExpired       = 0x05
)

type ProtocolError struct {
	code    uint64
	message string
}

func (e ProtocolError) Error() string {
	return e.message
}

func (e ProtocolError) Code() uint64 {
	return e.code
}

type ApplicationError struct {
	code   uint64
	mesage string
}

func (e ApplicationError) Error() string {
	return fmt.Sprintf("MoQ Application Error %v: %v", e.code, e.mesage)
}
