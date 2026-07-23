package wire2

import "github.com/mengelbart/moqtransport/varint"

type Location struct {
	Group  uint64
	Object uint64
}

func (l *Location) append(buf []byte) []byte {
	buf = varint.Append(buf, l.Group)
	return varint.Append(buf, l.Object)
}

func (l *Location) parse(data []byte) (int, error) {
	var n, parsed int
	var err error
	l.Group, n, err = varint.Parse(data)
	parsed += n
	if err != nil {
		return parsed, err
	}
	data = data[n:]

	l.Object, n, err = varint.Parse(data)
	parsed += n
	return parsed, err
}
