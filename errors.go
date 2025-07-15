package moqtransport

import "fmt"

// ErrorCode is a generic error codes
type ErrorCode uint64

const (
	ErrorCodeNoError                  ErrorCode = 0x00
	ErrorCodeInternal                 ErrorCode = 0x01
	ErrorCodeUnauthorized             ErrorCode = 0x02
	ErrorCodeProtocolViolation        ErrorCode = 0x03
	ErrorCodeInvalidRequestID         ErrorCode = 0x04
	ErrorCodeDuplicateTrackAlias      ErrorCode = 0x05
	ErrorCodeKeyValueFormattingError  ErrorCode = 0x06
	ErrorCodeTooManyRequests          ErrorCode = 0x07
	ErrorCodeInvalidPath              ErrorCode = 0x08
	ErrorCodeMalformedPath            ErrorCode = 0x09
	ErrorCodeGoAwayTimeout            ErrorCode = 0x10
	ErrorCodeControlMessageTimeout    ErrorCode = 0x11
	ErrorCodeDataStreamTimeout        ErrorCode = 0x12
	ErrorCodeAuthTokenCacheOverflow   ErrorCode = 0x13
	ErrorCodeDuplicateAuthTokenAlias  ErrorCode = 0x14
	ErrorCodeVersionNegotiationFailed ErrorCode = 0x15
	ErrorCodeMalformedAuthToken       ErrorCode = 0x16
	ErrorCodeUnknownAuthTokenAlias    ErrorCode = 0x17
	ErrorCodeExpiredAuthToken         ErrorCode = 0x18
)

// ErrorCodeSubscribe is a Subscribe error code
type ErrorCodeSubscribe uint64

const (
	ErrorCodeSubscribeInternal           ErrorCodeSubscribe = 0x00
	ErrorCodeSubscribeUnauthorized       ErrorCodeSubscribe = 0x01
	ErrorCodeSubscribeTimeout            ErrorCodeSubscribe = 0x02
	ErrorCodeSubscribeNotSupported       ErrorCodeSubscribe = 0x03
	ErrorCodeSubscribeTrackDoesNotExist  ErrorCodeSubscribe = 0x04
	ErrorCodeSubscribeInvalidRange       ErrorCodeSubscribe = 0x05
	ErrorCodeSubscribeMalformedAuthToken ErrorCodeSubscribe = 0x10
	ErrorCodeSubscribeExpiredAuthToken   ErrorCodeSubscribe = 0x12
)

// ErrorCodeSubscribeDone is a subscribe done error code
type ErrorCodeSubscribeDone uint64

const (
	ErrorCodeSubscribeDoneInternal          ErrorCodeSubscribeDone = 0x00
	ErrorCodeSubscribeDoneUnauthorized      ErrorCodeSubscribeDone = 0x01
	ErrorCodeSubscribeDoneTrackEnded        ErrorCodeSubscribeDone = 0x02
	ErrorCodeSubscribeDoneSubscriptionEnded ErrorCodeSubscribeDone = 0x03
	ErrorCodeSubscribeDoneGoingAway         ErrorCodeSubscribeDone = 0x04
	ErrorCodeSubscribeDoneExpired           ErrorCodeSubscribeDone = 0x05
	ErrorCodeSubscribeDoneTooFarBehind      ErrorCodeSubscribeDone = 0x06
	ErrorCodeSubscribeDoneMalformedTrack    ErrorCodeSubscribeDone = 0x07
)

// ErrorCodePublish is a publish error code
type ErrorCodePublish uint64

const (
	ErrorCodePublishInternalError ErrorCodePublish = 0x00
	ErrorCodePublishUnauthorized  ErrorCodePublish = 0x01
	ErrorCodePublishTimeout       ErrorCodePublish = 0x02
	ErrorCodePublishNotSupported  ErrorCodePublish = 0x03
	ErrorCodePublishUninterested  ErrorCodePublish = 0x04
)

// ErrorCodeFetch is a fetch error code
type ErrorCodeFetch uint64

const (
	ErrorCodeFetchInternal                  ErrorCodeFetch = 0x00
	ErrorCodeFetchUnauthorized              ErrorCodeFetch = 0x01
	ErrorCodeFetchTimeout                   ErrorCodeFetch = 0x02
	ErrorCodeFetchNotSupported              ErrorCodeFetch = 0x03
	ErrorCodeFetchTrackDoesNotExist         ErrorCodeFetch = 0x04
	ErrorCodeFetchInvalidRange              ErrorCodeFetch = 0x05
	ErrorCodeFetchNoObjects                 ErrorCodeFetch = 0x06
	ErrorCodeFetchInvalidJoiningSubscribeID ErrorCodeFetch = 0x07
	ErrorCodeFetchUnknownStatusInRange      ErrorCodeFetch = 0x08
	ErrorCodeFetchMalformedTrack            ErrorCodeFetch = 0x09
	ErrorCodeFetchMalformedAuthToken        ErrorCodeFetch = 0x10
	ErrorCodeFetchExpiredAuthToken          ErrorCodeFetch = 0x12
)

// ErrorCodeAnnounce is an announcement error code
type ErrorCodeAnnounce uint64

const (
	ErrorCodeAnnounceInternal             ErrorCodeAnnounce = 0x00
	ErrorCodeAnnounceUnauthorized         ErrorCodeAnnounce = 0x01
	ErrorCodeAnnounceTimeout              ErrorCodeAnnounce = 0x02
	ErrorCodeAnnounceNotSupported         ErrorCodeAnnounce = 0x03
	ErrorCodeAnnounceUninterested         ErrorCodeAnnounce = 0x04
	ErrorCodeAnnounceMalformedAuthToken   ErrorCodeAnnounce = 0x10
	ErrorCodeAnnouncementExpiredAuthToken ErrorCodeAnnounce = 0x12
)

// ErrorCodeSubscribeNamespace is a subscribe namespaces error code
type ErrorCodeSubscribeNamespace uint64

const (
	ErrorCodeSubscribeNamespaceInternal               ErrorCodeSubscribeNamespace = 0x00
	ErrorCodeSubscribeNamespaceUnauthorized           ErrorCodeSubscribeNamespace = 0x01
	ErrorCodeSubscribeNamespaceTimeout                ErrorCodeSubscribeNamespace = 0x02
	ErrorCodeSubscribeNamespaceNotSupported           ErrorCodeSubscribeNamespace = 0x03
	ErrorCodeSubscribeNamespaceNamespacePrefixUnknown ErrorCodeSubscribeNamespace = 0x04
	ErrorCodeSubscribeNamespaceNamespacePrefixOverlap ErrorCodeSubscribeNamespace = 0x05
	ErrorCodeSubscribeNamespaceMalformedAuthToken     ErrorCodeSubscribeNamespace = 0x10
	ErrorCodeSubscribeNamespaceExpiredAuthToken       ErrorCodeSubscribeNamespace = 0x12
)

// ProtocolError is a MoQ protocol error
type ProtocolError struct {
	code    ErrorCode
	message string
}

func (e *ProtocolError) String() string {
	return e.Error()
}

func (e ProtocolError) Error() string {
	return fmt.Sprintf("%v: %v", e.code, e.message)
}

func (e ProtocolError) Code() uint64 {
	return uint64(e.code)
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
	errInvalidNamespaceLength = ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "invalid namespace length",
	}
)
