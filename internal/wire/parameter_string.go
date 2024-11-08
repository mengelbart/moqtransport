package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

type StringParameter struct {
	Type  uint64
	Value string
}

func (r StringParameter) String() string {
	return fmt.Sprintf("{key: %v, value: '%v'}", r.Type, r.Value)
}

func (r StringParameter) key() uint64 {
	return r.Type
}

func (r StringParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.Type)
	buf = appendVarIntBytes(buf, []byte(r.Value))
	return buf
}

func parseStringParameter(data []byte, key uint64) (Parameter, int, error) {
	buf, n, err := parseVarIntBytes(data)
	if err != nil {
		return StringParameter{}, n, err
	}
	p := StringParameter{
		Type:  key,
		Value: string(buf),
	}
	return p, n, err
}
