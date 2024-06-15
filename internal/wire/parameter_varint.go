package wire

import (
	"fmt"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type VarintParameter struct {
	Type  uint64
	Value uint64
}

func (r VarintParameter) String() string {
	return fmt.Sprintf("key: %v, value: %v", r.Type, r.Value)
}

func (r VarintParameter) key() uint64 {
	return r.Type
}

func (r VarintParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.Type)
	buf = quicvarint.Append(buf, uint64(quicvarint.Len(r.Value)))
	buf = quicvarint.Append(buf, r.Value)
	return buf
}

func (p *VarintParameter) parse(reader io.ByteReader) (err error) {
	p.Value, err = quicvarint.Read(reader)
	return
}

func parseVarintParameter(reader io.ByteReader, typ uint64) (*VarintParameter, error) {
	p := &VarintParameter{
		Type: typ,
	}
	_, err := quicvarint.Read(reader)
	if err != nil {
		return nil, err
	}
	err = p.parse(reader)
	// TODO: p.parse should return the number of bytes it really read n. If that
	// number does not equal length returned by quicvarint.Read above, return
	// errParameterLengthMismatch
	// if n != length {
	// 	return nil, parsed, errParameterLengthMismatch
	// }
	return p, err
}
