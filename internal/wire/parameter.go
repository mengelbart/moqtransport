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

func (pp Parameters) parse(reader messageReader) error {
	numParameters, err := quicvarint.Read(reader)
	if err != nil {
		return err
	}
	for i := uint64(0); i < numParameters; i++ {
		p, err := parseParameter(reader)
		if err != nil {
			return err
		}
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

func parseParameter(reader messageReader) (p Parameter, err error) {
	var key uint64
	key, err = quicvarint.Read(reader)
	if err != nil {
		return nil, err
	}
	switch key {
	case RoleParameterKey:
		p, err = parseVarintParameter(reader, key)
		return p, err
	case PathParameterKey, AuthorizationParameterKey:
		p, err = parseStringParameter(reader, key)
		return p, err
	}
	length, err := quicvarint.Read(reader)
	if err != nil {
		return nil, err
	}
	_, err = reader.Discard(int(length))
	return nil, err
}
