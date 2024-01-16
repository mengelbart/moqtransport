package moqtransport

type errorCode uint64

const (
	noErrorErrorCode                 = 0x00
	genericErrorErrorCode            = 0x01
	unauthorizedErrorCode            = 0x02
	protocolViolationErrorCode       = 0x03
	duplicateTracksAliasErrorCode    = 0x04
	parameterLengthMismatchErrorCode = 0x05
	goAwayTimeoutErrorCode           = 0x10
)

var (
	errUnsupportedVersion = &moqError{
		code:    genericErrorErrorCode,
		message: "unsupported version",
	}
)

type moqError struct {
	code    errorCode
	message string
}

func (e *moqError) Error() string {
	return e.message
}
