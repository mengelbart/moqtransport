package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type announcement struct {
	requestID  uint64
	namespace  []string
	parameters wire.Parameters

	response chan error
}
