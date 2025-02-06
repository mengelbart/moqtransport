package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type announcement struct {
	Namespace  []string
	parameters wire.Parameters // TODO: This is unexported, need better API?

	response chan error
}
