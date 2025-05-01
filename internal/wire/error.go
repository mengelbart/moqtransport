package wire

import "errors"

var (
	errInvalidMessageType       = errors.New("invalid message type")
	errInvalidFilterType        = errors.New("invalid filter type")
	errInvalidContentExistsByte = errors.New("invalid use of ContentExists byte")
	errInvalidGroupOrder        = errors.New("invalid GroupOrder")
	errInvalidForwardFlag       = errors.New("invalid Forward flag")
	errLengthMismatch           = errors.New("length mismatch")
	errInvalidFetchType         = errors.New("invalid fetch type")
)
