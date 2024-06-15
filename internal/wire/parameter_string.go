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
	return fmt.Sprintf("key: %v, value: %v", r.Type, r.Value)
}

func (r StringParameter) key() uint64 {
	return r.Type
}

func (r StringParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.Type)
	buf = appendVarIntString(buf, r.Value)
	return buf
}

func (p *StringParameter) parse(reader messageReader) (err error) {
	p.Value, err = parseVarIntString(reader)
	return
}

func parseStringParameter(reader messageReader, key uint64) (*StringParameter, error) {
	p := &StringParameter{
		Type: key,
	}
	err := p.parse(reader)
	return p, err
}
