package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	PathParameterKey         = 0x01
	MaxRequestIDParameterKey = 0x02
)

const (
	AuthorizationParameterKey    = 0x02
	DeliveryTimeoutParameterKey  = 0x03
	MaxCacheDurationParameterKey = 0x04
)

type parameterParser struct {
	parse func([]byte, uint64, string) (Parameter, int, error)
	name  string
}

var unknownParameterParser = parameterParser{
	parse: parseUnknownParameter,
	name:  "unknown",
}

var setupParameterParsers = map[uint64]parameterParser{
	PathParameterKey: {
		parse: parseStringParameter,
		name:  "path",
	},
	MaxRequestIDParameterKey: {
		parse: parseVarintParameter,
		name:  "max_request_id",
	},
}

var versionSpecificParameterParsers = map[uint64]parameterParser{
	AuthorizationParameterKey: {
		parse: parseStringParameter,
		name:  "authorization_info",
	},
	DeliveryTimeoutParameterKey: {
		parse: parseVarintParameter,
		name:  "delivery_timeout",
	},
	MaxCacheDurationParameterKey: {
		parse: parseVarintParameter,
		name:  "max_cache_duration",
	},
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

func (pp Parameters) String() string {
	res := "["
	i := 0
	for _, v := range pp {
		if i < len(pp)-1 {
			res += fmt.Sprintf("%v, ", v)
		} else {
			res += fmt.Sprintf("%v", v)
		}
		i++
	}
	return res + "]"
}

func (pp Parameters) parseSetupParameters(data []byte) error {
	return pp.parseWithParser(data, setupParameterParsers)
}

func (pp Parameters) parseVersionSpecificParameters(data []byte) error {
	return pp.parseWithParser(data, versionSpecificParameterParsers)
}

func (pp Parameters) parseWithParser(data []byte, pm map[uint64]parameterParser) error {
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

func parseParameter(data []byte, pm map[uint64]parameterParser) (Parameter, int, error) {
	var p Parameter
	var n int
	var key uint64
	parsed := 0
	key, parsed, err := quicvarint.Parse(data)
	if err != nil {
		return nil, parsed, err
	}
	data = data[parsed:]

	parser, ok := pm[key]
	if !ok {
		parser = unknownParameterParser
	}
	p, n, err = parser.parse(data, key, parser.name)
	return p, parsed + n, err
}
