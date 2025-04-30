package moqtransport

// Generic error codes
const (
	ErrorCodeNoError                 uint64 = 0x00
	ErrorCodeInternal                uint64 = 0x01
	ErrorCodeUnauthorized            uint64 = 0x02
	ErrorCodeProtocolViolation       uint64 = 0x03
	ErrorCodeDuplicateTrackAlias     uint64 = 0x04
	ErrorCodeParameterLengthMismatch uint64 = 0x05
	ErrorCodeTooManySubscribes       uint64 = 0x06
	ErrorCodeGoAwayTimeout           uint64 = 0x10
	ErrorCodeControlMessageTimeout   uint64 = 0x11
	ErrorCodeDataStreamTimeout       uint64 = 0x12

	// Errors not included in current draft
	ErrorCodeUnsupportedVersion uint64 = 0xff01
)

// Announcement error codes
const (
	ErrorCodeAnnouncementInternalError uint64 = 0x00
	ErrorCodeAnnouncementUnauthorized  uint64 = 0x01
	ErrorCodeAnnouncementTimeout       uint64 = 0x02
	ErrorCodeAnnouncementNotSupported  uint64 = 0x03
	ErrorCodeAnnouncementUninterested  uint64 = 0x04
)

// Subscribe error codes
const (
	ErrorCodeSubscribeInternal          uint64 = 0x00
	ErrorCodeSubscribeUnauthorized      uint64 = 0x01
	ErrorCodeSubscribeTimeout           uint64 = 0x02
	ErrorCodeSubscribeNotSupported      uint64 = 0x03
	ErrorCodeSubscribeTrackDoesNotExist uint64 = 0x04
	ErrorCodeSubscribeInvalidRange      uint64 = 0x05
	ErrorCodeSubscribeRetryTrackAlias   uint64 = 0x06
)

// Fetch error codes
const (
	ErrorCodeFetchInternalError     uint64 = 0x00
	ErrorCodeFetchUnauthorized      uint64 = 0x01
	ErrorCodeFetchTimeout           uint64 = 0x02
	ErrorCodeFetchNotSupported      uint64 = 0x03
	ErrorCodeFetchTrackDoesNotExist uint64 = 0x04
	ErrorCodeFetchInvalidRange      uint64 = 0x05
)

// Subscribe done error codes
const (
	ErrorCodeSubscribeDoneInternalError     uint64 = 0x00
	ErrorCodeSubscribeDoneUnauthorized      uint64 = 0x01
	ErrorCodeSubscribeDoneTrackEnded        uint64 = 0x02
	ErrorCodeSubscribeDoneSubscriptionEnded uint64 = 0x03
	ErrorCodeSubscribeDoneGoingAway         uint64 = 0x04
	ErrorCodeSubscribeDoneExpired           uint64 = 0x05
	ErrorCodeSubscribeDoneTooFarBehind      uint64 = 0x06
)

// Subscribe Announces error codes
const (
	ErrorCodeSubscribeAnnouncesInternalError          uint64 = 0x00
	ErrorCodeSubscribeAnnouncesUnauthorized           uint64 = 0x01
	ErrorCodeSubscribeAnnouncesTimeout                uint64 = 0x02
	ErrorCodeSubscribeAnnouncesNotSupported           uint64 = 0x03
	ErrorCodeSubscribeAnnouncesNamespacePrefixUnknown uint64 = 0x04
)

// ProtocolError is a MoQ protocol error
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
