package moqtransport

import "fmt"

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
	code   uint64
	mesage string
}

func (e ApplicationError) Error() string {
	return fmt.Sprintf("MoQ Application Error %v: %v", e.code, e.mesage)
}
