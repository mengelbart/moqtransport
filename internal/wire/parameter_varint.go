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

func parseVarintParameter(data []byte, typ uint64) (VarintParameter, int, error) {
	parsed := 0
	l, n, err := quicvarint.Parse(data)
	if err != nil {
		return VarintParameter{}, n, err
	}
	parsed += n
	data = data[n:]

	v, n, err := quicvarint.Parse(data)
	if err != nil {
		return VarintParameter{}, n, err
	}
	parsed += n

	if n != int(l) {
		return VarintParameter{}, n + int(l), errLengthMismatch
	}
	return VarintParameter{
		Type:  typ,
		Value: v,
	}, parsed, nil
}
