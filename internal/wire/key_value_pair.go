package wire

import (
	"errors"

	"github.com/mengelbart/moqtransport/varint"
)

var errLengthMismatch = errors.New("length mismatch")

type KeyValuePair struct {
	Type   uint64
	Bytes  []byte
	Varint uint64
}

// func (p KeyValuePair) length() uint64 {
// 	length := uint64(varint.Len(p.Type))
// 	if p.Type%2 == 1 {
// 		length += uint64(varint.Len(uint64(len(p.Bytes))))
// 		length += uint64(len(p.Bytes))
// 		return length
// 	}
// 	length += uint64(varint.Len(p.Varint))
// 	return length
// }

func (p *KeyValuePair) append(buf []byte) []byte {
	buf = varint.Append(buf, p.Type)
	if p.Type%2 == 1 {
		buf = varint.Append(buf, uint64(len(p.Bytes)))
		return append(buf, p.Bytes...)
	}
	return varint.Append(buf, p.Varint)
}

func (p *KeyValuePair) parse(data []byte) (int, error) {
	var n, parsed int
	var err error
	p.Type, n, err = varint.Parse(data)
	parsed += n
	if err != nil {
		return n, err
	}
	data = data[n:]

	if p.Type%2 == 1 {
		var length uint64
		length, n, err = varint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
		data = data[n:]
		p.Bytes = make([]byte, length) // TODO: Don't allocate memory here?
		m := copy(p.Bytes, data[:length])
		parsed += m
		if uint64(m) != length {
			return parsed, errLengthMismatch
		}
		return parsed, nil
	}

	p.Varint, n, err = varint.Parse(data)
	parsed += n
	return parsed, err
}

// func (p *KeyValuePair) parseReader(br *bufio.Reader) error {
// 	var err error
// 	p.Type, err = varint.Read(br)
// 	if err != nil {
// 		return err
// 	}
// 	if p.Type%2 == 1 {
// 		var length uint64
// 		length, err = varint.Read(br)
// 		if err != nil {
// 			return err
// 		}
// 		p.Bytes = make([]byte, length)
// 		var m int
// 		m, err = io.ReadFull(br, p.Bytes)
// 		if err != nil {
// 			return err
// 		}
// 		if uint64(m) != length {
// 			return errLengthMismatch
// 		}
// 		return nil
// 	}
// 	p.Varint, err = varint.Read(br)
// 	return err
// }
