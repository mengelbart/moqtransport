package moqtransport

// Generic error codes
const (
	ErrorCodeNoError                 = 0x00
	ErrorCodeInternal                = 0x01
	ErrorCodeUnauthorized            = 0x02
	ErrorCodeProtocolViolation       = 0x03
	ErrorCodeDuplicateTrackAlias     = 0x04
	ErrorCodeParameterLengthMismatch = 0x05
	ErrorCodeTooManySubscribes       = 0x06
	ErrorCodeGoAwayTimeout           = 0x10
	ErrorCodeControlMessageTimeout   = 0x11
	ErrorCodeDataStreamTimeout       = 0x12

	// Errors not included in current draft
	ErrorCodeUnsupportedVersion = 0xff01
)

// Announcement error codes
const (
	ErrorCodeAnnouncementInternalError = 0x00
	ErrorCodeAnnouncementUnauthorized  = 0x01
	ErrorCodeAnnouncementTimeout       = 0x02
	ErrorCodeAnnouncementNotSupported  = 0x03
	ErrorCodeAnnouncementUninterested  = 0x04
)

// Subscribe error codes
const (
	ErrorCodeSubscribeInternal          = 0x00
	ErrorCodeSubscribeUnauthorized      = 0x01
	ErrorCodeSubscribeTimeout           = 0x02
	ErrorCodeSubscribeNotSupported      = 0x03
	ErrorCodeSubscribeTrackDoesNotExist = 0x04
	ErrorCodeSubscribeInvalidRange      = 0x05
	ErrorCodeSubscribeRetryTrackAlias   = 0x06
)

// Fetch error codes
const (
	ErrorCodeFetchInternalError     = 0x00
	ErrorCodeFetchUnauthorized      = 0x01
	ErrorCodeFetchTimeout           = 0x02
	ErrorCodeFetchNotSupported      = 0x03
	ErrorCodeFetchTrackDoesNotExist = 0x04
	ErrorCodeFetchInvalidRange      = 0x05
)

// Subscribe done error codes
const (
	ErrorCodeSubscribeDoneInternalError     = 0x00
	ErrorCodeSubscribeDoneUnauthorized      = 0x01
	ErrorCodeSubscribeDoneTrackEnded        = 0x02
	ErrorCodeSubscribeDoneSubscriptionEnded = 0x03
	ErrorCodeSubscribeDoneGoingAway         = 0x04
	ErrorCodeSubscribeDoneExpired           = 0x05
	ErrorCodeSubscribeDoneTooFarBehind      = 0x06
)

// Subscribe Announces error codes
const (
	ErrorCodeSubscribeAnnouncesInternalError          = 0x00
	ErrorCodeSubscribeAnnouncesUnauthorized           = 0x01
	ErrorCodeSubscribeAnnouncesTimeout                = 0x02
	ErrorCodeSubscribeAnnouncesNotSupported           = 0x03
	ErrorCodeSubscribeAnnouncesNamespacePrefixUnknown = 0x04
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
	errDuplicateSubscribeID = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "duplicate subscribe ID",
	}
	errMaxSubscribeIDDecreased = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "max subscribe ID decreased",
	}
	errUnknownSubscribeID = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unknown subscribe ID",
	}
	errDuplicateAnnouncementNamespace = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "duplicate announcement namespace",
	}
	errUnknownAnnouncement = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unknown announcement",
	}
)
