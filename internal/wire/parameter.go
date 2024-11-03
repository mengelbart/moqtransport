package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	RoleParameterKey uint64 = iota
	PathParameterKey
	AuthorizationParameterKey
)

type Parameter interface {
	append([]byte) []byte
	key() uint64
	String() string
}

type Parameters map[uint64]Parameter

func (p Parameters) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(p)))
	for _, p := range p {
		buf = p.append(buf)
	}
	return buf
}

func (p Parameters) String() string {
	res := ""
	for k, v := range p {
		res += fmt.Sprintf("%v - {%v}\n", k, v.String())
	}
	return res
}

func (pp Parameters) parse(data []byte) error {
	numParameters, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	for i := uint64(0); i < numParameters; i++ {
		p, n, err := parseParameter(data)
		if err != nil {
			return err
		}
		data = data[n:]
		if p == nil {
			continue
		}
		if _, ok := pp[p.key()]; ok {
			return errDuplicateParameter
		}
		pp[p.key()] = p
	}
	return nil
}

func parseParameter(data []byte) (Parameter, int, error) {
	var p Parameter
	var n int
	var key uint64
	parsed := 0
	key, parsed, err := quicvarint.Parse(data)
	if err != nil {
		return nil, parsed, err
	}
	data = data[parsed:]

	switch key {
	case RoleParameterKey:
		p, n, err = parseVarintParameter(data, key)
		return p, parsed + n, err
	case PathParameterKey, AuthorizationParameterKey:
		p, n, err = parseStringParameter(data, key)
		return p, parsed + n, err
	default:
		l, n, err := quicvarint.Parse(data)
		return nil, parsed + n + int(l), err
	}
}
