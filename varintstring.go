package moqtransport

import (
	"io"

	"gitlab.lrz.de/cm/moqtransport/varint"
)

func appendVarIntString(buf []byte, s string) []byte {
	buf = varint.Append(buf, uint64(len(s)))
	buf = append(buf, s...)
	return buf
}

func varIntStringLen(s string) uint64 {
	return uint64(varint.Len(uint64(len(s)))) + uint64(len(s))
}

func parseVarIntString(r MessageReader) (string, int, error) {
	if r == nil {
		return "", 0, errInvalidMessageReader
	}
	l, n, err := varint.ReadWithLen(r)
	if err != nil {
		return "", 0, err
	}
	if l == 0 {
		return "", n, nil
	}
	val := make([]byte, l)
	var m int
	m, err = r.Read(val)
	if err != nil {
		return "", n + m, err
	}
	if uint64(m) != l {
		return "", 0, io.EOF
	}
	return string(val), n + m, nil
}
