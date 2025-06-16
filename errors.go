package moqtransport

import "fmt"

// Generic error codes
const (
	ErrorCodeNoError                  uint64 = 0x00
	ErrorCodeInternal                 uint64 = 0x01
	ErrorCodeUnauthorized             uint64 = 0x02
	ErrorCodeProtocolViolation        uint64 = 0x03
	ErrorCodeInvalidRequestID         uint64 = 0x04
	ErrorCodeDuplicateTrackAlias      uint64 = 0x05
	ErrorCodeKeyValueFormattingError  uint64 = 0x06
	ErrorCodeTooManyRequests          uint64 = 0x07
	ErrorCodeInvalidPath              uint64 = 0x08
	ErrorCodeMalformedPath            uint64 = 0x09
	ErrorCodeGoAwayTimeout            uint64 = 0x10
	ErrorCodeControlMessageTimeout    uint64 = 0x11
	ErrorCodeDataStreamTimeout        uint64 = 0x12
	ErrorCodeAuthTokenCacheOverflow   uint64 = 0x13
	ErrorCodeDuplicateAuthTokenAlias  uint64 = 0x14
	ErrorCodeVersionNegotiationFailed uint64 = 0x15
)

// Subscribe error codes
const (
	ErrorCodeSubscribeInternal              uint64 = 0x00
	ErrorCodeSubscribeUnauthorized          uint64 = 0x01
	ErrorCodeSubscribeTimeout               uint64 = 0x02
	ErrorCodeSubscribeNotSupported          uint64 = 0x03
	ErrorCodeSubscribeTrackDoesNotExist     uint64 = 0x04
	ErrorCodeSubscribeInvalidRange          uint64 = 0x05
	ErrorCodeSubscribeRetryTrackAlias       uint64 = 0x06
	ErrorCodeSubscribeMalformedAuthToken    uint64 = 0x10
	ErrorCodeSubscribeUnknownAuthTokenAlias uint64 = 0x11
	ErrorCodeSubscribeExpiredAuthToken      uint64 = 0x12
)

// Subscribe done error codes
const (
	ErrorCodeSubscribeDoneInternal          uint64 = 0x00
	ErrorCodeSubscribeDoneUnauthorized      uint64 = 0x01
	ErrorCodeSubscribeDoneTrackEnded        uint64 = 0x02
	ErrorCodeSubscribeDoneSubscriptionEnded uint64 = 0x03
	ErrorCodeSubscribeDoneGoingAway         uint64 = 0x04
	ErrorCodeSubscribeDoneExpired           uint64 = 0x05
	ErrorCodeSubscribeDoneTooFarBehind      uint64 = 0x06
)

// Fetch error codes
const (
	ErrorCodeFetchInternal                  uint64 = 0x00
	ErrorCodeFetchUnauthorized              uint64 = 0x01
	ErrorCodeFetchTimeout                   uint64 = 0x02
	ErrorCodeFetchNotSupported              uint64 = 0x03
	ErrorCodeFetchTrackDoesNotExist         uint64 = 0x04
	ErrorCodeFetchNoObjects                 uint64 = 0x06
	ErrorCodeFetchInvalidJoiningSubscribeID uint64 = 0x07
	ErrorCodeFetchMalformedAuthToken        uint64 = 0x10
	ErrorCodeFetchUnknownAuthTokenAlias     uint64 = 0x11
	ErrorCodeFetchExpiredAuthToken          uint64 = 0x12
)

// Announcement error codes
const (
	ErrorCodeAnnouncementInternal              uint64 = 0x00
	ErrorCodeAnnouncementUnauthorized          uint64 = 0x01
	ErrorCodeAnnouncementTimeout               uint64 = 0x02
	ErrorCodeAnnouncementNotSupported          uint64 = 0x03
	ErrorCodeAnnouncementUninterested          uint64 = 0x04
	ErrorCodeAnnouncementMalformedAuthToken    uint64 = 0x10
	ErrorCodeAnnouncementUnknownAuthTokenAlias uint64 = 0x11
	ErrorCodeAnnouncementExpiredAuthToken      uint64 = 0x12
)

// Subscribe Announces error codes
const (
	ErrorCodeSubscribeAnnouncesInternal               uint64 = 0x00
	ErrorCodeSubscribeAnnouncesUnauthorized           uint64 = 0x01
	ErrorCodeSubscribeAnnouncesTimeout                uint64 = 0x02
	ErrorCodeSubscribeAnnouncesNotSupported           uint64 = 0x03
	ErrorCodeSubscribeAnnouncesNamespacePrefixUnknown uint64 = 0x04
	ErrorCodeSubscribeAnnouncesNamespacePrefixOverlap uint64 = 0x05
	ErrorCodeSubscribeAnnouncesMalformedAuthToken     uint64 = 0x10
	ErrorCodeSubscribeAnnouncesUnknownAuthTokenAlias  uint64 = 0x11
	ErrorCodeSubscribeAnnouncesExpiredAuthToken       uint64 = 0x12
)

// ProtocolError is a MoQ protocol error
type ProtocolError struct {
	code    uint64
	message string
}

func (e *ProtocolError) String() string {
	return e.Error()
}

func (e ProtocolError) Error() string {
	return fmt.Sprintf("%v: %v", e.code, e.message)
}

func (e ProtocolError) Code() uint64 {
	return e.code
}

var (
	errDuplicateRequestID = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "duplicate request ID",
	}
	errMaxRequestIDDecreased = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "max request ID decreased",
	}
	errUnknownRequestID = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unknown request ID",
	}
	errUnknownAnnouncement = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unknown announcement",
	}
)
