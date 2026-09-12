package moqtransport

import (
	"errors"
	"fmt"
	"time"
)

// ErrRequestClosed is returned when a request stream ends before the peer
// answered the request.
var ErrRequestClosed = errors.New("request stream closed without response")

// Redirect carries the target of a REQUEST_ERROR with code REDIRECT.
type Redirect struct {
	ConnectURI string
	Namespace  [][]byte
	Name       []byte
}

// RequestError is a REQUEST_ERROR received from the peer.
type RequestError struct {
	Code          RequestErrorCode
	Reason        string
	RetryInterval uint64
	Redirect      *Redirect
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("request error %#x: %s", uint64(e.Code), e.Reason)
}

func (e *RequestError) Is(target error) bool {
	other, ok := target.(*RequestError)
	return ok && e.Code == other.Code
}

// RetryAfter returns the interval after which the request may be retried.
// The second return value is false when the peer asked not to retry.
func (e *RequestError) RetryAfter() (time.Duration, bool) {
	if e.RetryInterval == 0 {
		return 0, false
	}
	return time.Duration(e.RetryInterval-1) * time.Millisecond, true
}

// ErrorCode is a session termination error code.
type ErrorCode uint64

const (
	ErrorCodeNoError                  ErrorCode = 0x00
	ErrorCodeInternal                 ErrorCode = 0x01
	ErrorCodeUnauthorized             ErrorCode = 0x02
	ErrorCodeProtocolViolation        ErrorCode = 0x03
	ErrorCodeInvalidRequestID         ErrorCode = 0x04
	ErrorCodeDuplicateTrackAlias      ErrorCode = 0x05
	ErrorCodeKeyValueFormattingError  ErrorCode = 0x06
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
	ErrorCodeInvalidAuthority         ErrorCode = 0x19
	ErrorCodeMalformedAuthority       ErrorCode = 0x1A
)

// RequestErrorCode is a REQUEST_ERROR error code.
type RequestErrorCode uint64

const (
	RequestErrorCodeInternal                RequestErrorCode = 0x00
	RequestErrorCodeUnauthorized            RequestErrorCode = 0x01
	RequestErrorCodeTimeout                 RequestErrorCode = 0x02
	RequestErrorCodeNotSupported            RequestErrorCode = 0x03
	RequestErrorCodeMalformedAuthToken      RequestErrorCode = 0x04
	RequestErrorCodeExpiredAuthToken        RequestErrorCode = 0x05
	RequestErrorCodeGoingAway               RequestErrorCode = 0x06
	RequestErrorCodeExcessiveLoad           RequestErrorCode = 0x09
	RequestErrorCodeDoesNotExist            RequestErrorCode = 0x10
	RequestErrorCodeInvalidRange            RequestErrorCode = 0x11
	RequestErrorCodeMalformedTrack          RequestErrorCode = 0x12
	RequestErrorCodeDuplicateSubscription   RequestErrorCode = 0x19
	RequestErrorCodeUninterested            RequestErrorCode = 0x20
	RequestErrorCodePrefixOverlap           RequestErrorCode = 0x30
	RequestErrorCodeNamespaceTooLarge       RequestErrorCode = 0x31
	RequestErrorCodeInvalidJoiningRequestID RequestErrorCode = 0x32
	RequestErrorCodeUnsupportedExtension    RequestErrorCode = 0x33
	RequestErrorCodeRedirect                RequestErrorCode = 0x34
)

// StreamResetErrorCode is used when resetting a stream or sending STOP_SENDING.
type StreamResetErrorCode uint32

const (
	StreamResetErrorCodeInternal            StreamResetErrorCode = 0x00
	StreamResetErrorCodeCancelled           StreamResetErrorCode = 0x01
	StreamResetErrorCodeDeliveryTimeout     StreamResetErrorCode = 0x02
	StreamResetErrorCodeSessionClosed       StreamResetErrorCode = 0x03
	StreamResetErrorCodeGoingAway           StreamResetErrorCode = 0x04
	StreamResetErrorCodeTooFarBehind        StreamResetErrorCode = 0x05
	StreamResetErrorCodeUnknownObjectStatus StreamResetErrorCode = 0x06
	StreamResetErrorCodeExpiredAuthToken    StreamResetErrorCode = 0x07
	StreamResetErrorCodeExcessiveLoad       StreamResetErrorCode = 0x09
	StreamResetErrorCodeMalformedTrack      StreamResetErrorCode = 0x12
)

// PublishDoneStatusCode says why a subscription ended.
type PublishDoneStatusCode uint64

const (
	PublishDoneStatusCodeInternal          PublishDoneStatusCode = 0x00
	PublishDoneStatusCodeUnauthorized      PublishDoneStatusCode = 0x01
	PublishDoneStatusCodeTrackEnded        PublishDoneStatusCode = 0x02
	PublishDoneStatusCodeSubscriptionEnded PublishDoneStatusCode = 0x03
	PublishDoneStatusCodeGoingAway         PublishDoneStatusCode = 0x04
	PublishDoneStatusCodeTooFarBehind      PublishDoneStatusCode = 0x05
	PublishDoneStatusCodeExpired           PublishDoneStatusCode = 0x06
	PublishDoneStatusCodeUpdateFailed      PublishDoneStatusCode = 0x08
	PublishDoneStatusCodeExcessiveLoad     PublishDoneStatusCode = 0x09
	PublishDoneStatusCodeMalformedTrack    PublishDoneStatusCode = 0x12
)
