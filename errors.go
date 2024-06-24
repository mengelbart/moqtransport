package moqtransport

import "fmt"

const (
	ErrorCodeNoError                 = 0x00
	ErrorCodeInternal                = 0x01
	ErrorCodeUnauthorized            = 0x02
	ErrorCodeProtocolViolation       = 0x03
	ErrorCodeDuplicateTrackAlias     = 0x04
	ErrorCodeParameterLengthMismatch = 0x05
	ErrorCodeGoAwayTimeout           = 0x10

	// Errors not included in current draft
	ErrorCodeUnsupportedVersion = 0xff01
	ErrorCodeTrackNotFound      = 0xff02
)

const (
	SubscribeErrorInternal        = 0x00
	SubscribeErrorInvalidRange    = 0x01
	SubscribeErrorRetryTrackAlias = 0x02

	// TODO: These are not specified yet, but seem useful
	SubscribeErrorUnknownTrack = 0x03
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
	Code   uint64
	Mesage string
}

func (e ApplicationError) Error() string {
	return fmt.Sprintf("MoQ Application Error %v: %v", e.Code, e.Mesage)
}
