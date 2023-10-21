package moqtransport

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

func appendVarIntString(buf []byte, s string) []byte {
	buf = quicvarint.Append(buf, uint64(len(s)))
	buf = append(buf, s...)
	return buf
}

func varIntStringLen(s string) uint64 {
	return uint64(quicvarint.Len(uint64(len(s)))) + uint64(len(s))
}

func parseVarIntString(r messageReader) (string, error) {
	if r == nil {
		return "", errInvalidMessageReader
	}
	l, err := quicvarint.Read(r)
	if err != nil {
		return "", err
	}
	if l == 0 {
		return "", nil
	}
	val := make([]byte, l)
	var m int
	m, err = r.Read(val)
	if err != nil {
		return "", err
	}
	if uint64(m) != l {
		return "", io.EOF
	}
	return string(val), nil
}
