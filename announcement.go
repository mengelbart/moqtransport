package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type announcement struct {
	namespace  []string
	parameters wire.Parameters

	response chan error
}
