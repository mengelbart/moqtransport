package wire

import "github.com/quic-go/quic-go/quicvarint"

type Location struct {
	Group  uint64
	Object uint64
}

func (l Location) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, l.Group)
	return quicvarint.Append(buf, l.Object)
}

func (l *Location) parse(_ Version, data []byte) (n int, err error) {
	l.Group, n, err = quicvarint.Parse(data)
	if err != nil {
		return n, err
	}
	data = data[n:]
	var m int
	l.Object, m, err = quicvarint.Parse(data)
	return n + m, err
}
