package wire

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

func appendVarIntBytes(buf []byte, data []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(data)))
	buf = append(buf, data...)
	return buf
}

func varIntBytesLen(s string) uint64 {
	return uint64(quicvarint.Len(uint64(len(s)))) + uint64(len(s))
}

func parseVarIntBytes(data []byte) ([]byte, int, error) {
	l, n, err := quicvarint.Parse(data)
	if err != nil {
		return []byte{}, n, err
	}

	if l == 0 {
		return []byte{}, n, nil
	}
	data = data[n:]

	if len(data) < int(l) {
		return []byte{}, n + len(data), io.ErrUnexpectedEOF
	}
	return data[:l], n + int(l), nil
}
