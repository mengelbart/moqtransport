package wire

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

func parseVarIntString(reader messageReader) (string, error) {
	l, err := quicvarint.Read(reader)
	if err != nil {
		return "", err
	}
	if l == 0 {
		return "", nil
	}
	buf := make([]byte, l)
	n, err := reader.Read(buf)
	if err != nil {
		return "", err
	}
	if n < int(l) {
		return "", io.ErrUnexpectedEOF
	}
	return string(buf[:n]), nil
}
