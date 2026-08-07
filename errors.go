package moqtransport

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

// SubscribeErrorCode is a Subscribe error code
type SubscribeErrorCode uint64

const (
	SubscribeErrorCodeInternal           SubscribeErrorCode = 0x00
	SubscribeErrorCodeUnauthorized       SubscribeErrorCode = 0x01
	SubscribeErrorCodeTimeout            SubscribeErrorCode = 0x02
	SubscribeErrorCodeNotSupported       SubscribeErrorCode = 0x03
	SubscribeErrorCodeTrackDoesNotExist  SubscribeErrorCode = 0x04
	SubscribeErrorCodeInvalidRange       SubscribeErrorCode = 0x05
	SubscribeErrorCodeMalformedAuthToken SubscribeErrorCode = 0x10
	SubscribeErrorCodeExpiredAuthToken   SubscribeErrorCode = 0x12
)

// SubscribeErrorCodeDone is a subscribe done error code
type SubscribeErrorCodeDone uint64

const (
	SubscribeErrorCodeDoneInternal          SubscribeErrorCodeDone = 0x00
	SubscribeErrorCodeDoneUnauthorized      SubscribeErrorCodeDone = 0x01
	SubscribeErrorCodeDoneTrackEnded        SubscribeErrorCodeDone = 0x02
	SubscribeErrorCodeDoneSubscriptionEnded SubscribeErrorCodeDone = 0x03
	SubscribeErrorCodeDoneGoingAway         SubscribeErrorCodeDone = 0x04
	SubscribeErrorCodeDoneExpired           SubscribeErrorCodeDone = 0x05
	SubscribeErrorCodeDoneTooFarBehind      SubscribeErrorCodeDone = 0x06
	SubscribeErrorCodeDoneMalformedTrack    SubscribeErrorCodeDone = 0x07
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

// SubscribeErrorCodeNamespace is a subscribe namespaces error code
type SubscribeErrorCodeNamespace uint64

const (
	SubscribeErrorCodeNamespaceInternal               SubscribeErrorCodeNamespace = 0x00
	SubscribeErrorCodeNamespaceUnauthorized           SubscribeErrorCodeNamespace = 0x01
	SubscribeErrorCodeNamespaceTimeout                SubscribeErrorCodeNamespace = 0x02
	SubscribeErrorCodeNamespaceNotSupported           SubscribeErrorCodeNamespace = 0x03
	SubscribeErrorCodeNamespaceNamespacePrefixUnknown SubscribeErrorCodeNamespace = 0x04
	SubscribeErrorCodeNamespaceNamespacePrefixOverlap SubscribeErrorCodeNamespace = 0x05
	SubscribeErrorCodeNamespaceMalformedAuthToken     SubscribeErrorCodeNamespace = 0x10
	SubscribeErrorCodeNamespaceExpiredAuthToken       SubscribeErrorCodeNamespace = 0x12
)
