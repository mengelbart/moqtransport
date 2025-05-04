package wire

import "github.com/quic-go/quic-go/quicvarint"

const (
	TokenTypeDelete   = 0x00
	TokenTypeRegister = 0x01
	TokenTypeUseAlias = 0x02
	TokenTypeUseValue = 0x03
)

type Token struct {
	AliasType uint64
	Alias     uint64
	Type      uint64
	Value     []byte
}

func (t Token) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, t.AliasType)
	switch t.AliasType {
	case TokenTypeDelete, TokenTypeUseAlias:
		buf = quicvarint.Append(buf, t.Alias)
	case TokenTypeRegister:
		buf = quicvarint.Append(buf, t.Alias)
		buf = quicvarint.Append(buf, t.Type)
		buf = append(buf, t.Value...)
	case TokenTypeUseValue:
		buf = quicvarint.Append(buf, t.Type)
		buf = append(buf, t.Value...)
	}
	return buf
}

func (t *Token) Parse(data []byte) (parsed int, err error) {
	var n int
	t.AliasType, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return parsed, err
	}
	data = data[n:]

	switch t.AliasType {
	case TokenTypeDelete, TokenTypeUseAlias:
		t.Alias, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}

	case TokenTypeRegister:
		t.Alias, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
		data = data[n:]

		t.Type, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
		data = data[n:]

		t.Value = make([]byte, len(data))
		n = copy(t.Value, data)
		parsed += n
		if n != len(data) {
			return parsed, errLengthMismatch
		}

	case TokenTypeUseValue:
		t.Type, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
		data = data[n:]

		t.Value = make([]byte, len(data))
		n = copy(t.Value, data)
		parsed += n
		if n != len(data) {
			return parsed, errLengthMismatch
		}
	}
	return
}
