package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	RoleParameterKey uint64 = iota
	PathParameterKey
	MaxSubscribeIDParameterKey
)

const (
	AuthorizationParameterKey uint64 = iota + 2
	DeliveryTimeoutParameterKey
	MaxCacheDurationParameterKey
)

type parameterParserFunc func([]byte, uint64) (Parameter, int, error)

var setupParameterTypes = map[uint64]parameterParserFunc{
	RoleParameterKey:           parseVarintParameter,
	PathParameterKey:           parseStringParameter,
	MaxSubscribeIDParameterKey: parseVarintParameter,
}

var versionSpecificParameterTypes = map[uint64]parameterParserFunc{
	AuthorizationParameterKey:    parseStringParameter,
	DeliveryTimeoutParameterKey:  parseVarintParameter,
	MaxCacheDurationParameterKey: parseVarintParameter,
}

type Parameter interface {
	append([]byte) []byte
	key() uint64
	String() string
}

type Parameters map[uint64]Parameter

func (pp Parameters) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(pp)))
	for _, p := range pp {
		buf = p.append(buf)
	}
	return buf
}

func (p Parameters) String() string {
	res := "["
	i := 0
	for _, v := range p {
		if i < len(p)-1 {
			res += fmt.Sprintf("%v, ", v)
		} else {
			res += fmt.Sprintf("%v", v)
		}
		i++
	}
	return res + "]"
}

func (pp Parameters) parse(data []byte, pm map[uint64]parameterParserFunc) error {
	numParameters, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	for i := uint64(0); i < numParameters; i++ {
		p, n, err := parseParameter(data, pm)
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

func parseParameter(data []byte, pm map[uint64]parameterParserFunc) (Parameter, int, error) {
	var p Parameter
	var n int
	var key uint64
	var l uint64
	parsed := 0
	key, parsed, err := quicvarint.Parse(data)
	if err != nil {
		return nil, parsed, err
	}
	data = data[parsed:]

	parser, ok := pm[key]
	if !ok {
		l, n, err = quicvarint.Parse(data)
		return nil, parsed + n + int(l), err
	}
	p, n, err = parser(data, key)
	return p, parsed + n, err
}
