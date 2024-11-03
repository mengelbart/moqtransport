package wire

import (
	"bytes"

	"github.com/quic-go/quic-go/quicvarint"
)

type Tuple [][]byte

func (t Tuple) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(t)))
	for _, t := range t {
		buf = quicvarint.Append(buf, uint64(len(t)))
		buf = append(buf, t...)
	}
	return buf
}

func (t Tuple) Equal(other Tuple) bool {
	if len(t) != len(other) {
		return false
	}
	for i, t := range t {
		if !bytes.Equal(t, other[i]) {
			return false
		}
	}
	return true
}

func (t Tuple) String() string {
	res := ""
	for _, t := range t {
		res += string(t)
	}
	return res
}

func parseTuple(data []byte) (Tuple, int, error) {
	length, parsed, err := quicvarint.Parse(data)
	if err != nil {
		return nil, parsed, err
	}
	data = data[parsed:]

	tuple := make([][]byte, 0, length)
	for i := uint64(0); i < length; i++ {
		l, n, err := quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return tuple, parsed, err
		}
		data = data[n:]

		if uint64(len(data)) < l {
			return tuple, parsed, errLengthMismatch
		}
		tuple = append(tuple, data[:l])
		data = data[l:]
		parsed += int(l)
	}
	return tuple, parsed, nil
}
