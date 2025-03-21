package wire

import "errors"

var (
	errInvalidMessageType       = errors.New("invalid message type")
	errInvalidFilterType        = errors.New("invalid filter type")
	errDuplicateParameter       = errors.New("duplicated parameter")
	errInvalidContentExistsByte = errors.New("invalid use of ContentExists byte")
	errInvalidGroupOrder        = errors.New("invalid GroupOrder")
	errLengthMismatch           = errors.New("length mismatch")
	errInvalidFetchType         = errors.New("invalid fetch type")
)
