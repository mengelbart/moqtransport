package wire

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mengelbart/moqtransport/varint"
)

const (
	PathParameterKey               = 0x01
	MaxRequestIDParameterKey       = 0x02
	AuthorizationTokenParameterKey = 0x03
)

const (
	ParameterTypeObjectDeliveryTimeout   uint64 = 0x02
	ParameterTypeAuthorizationToken      uint64 = 0x03
	ParameterTypeRendezvousTimeout       uint64 = 0x04
	ParameterTypeSubgroupDeliveryTimeout uint64 = 0x06
	ParameterTypeExpires                 uint64 = 0x08
	ParameterTypeLargestObject           uint64 = 0x09
	ParameterTypeFillTimeout             uint64 = 0x0A
	ParameterTypeForward                 uint64 = 0x10
	ParameterTypeSubscriberPriority      uint64 = 0x20
	ParameterTypeSubscriptionFilter      uint64 = 0x21
	ParameterTypeGroupOrder              uint64 = 0x22
	ParameterTypeNewGroupRequest         uint64 = 0x32
	ParameterTypeTrackNamespacePrefix    uint64 = 0x34
)

type ParameterEncoding uint8

const (
	ParameterEncodingUint8 ParameterEncoding = iota + 1
	ParameterEncodingVarint
	ParameterEncodingLocation
	ParameterEncodingBytes
	ParameterEncodingNamespace
)

var parameterEncodings = map[uint64]ParameterEncoding{
	ParameterTypeObjectDeliveryTimeout:   ParameterEncodingVarint,
	ParameterTypeAuthorizationToken:      ParameterEncodingBytes,
	ParameterTypeRendezvousTimeout:       ParameterEncodingVarint,
	ParameterTypeSubgroupDeliveryTimeout: ParameterEncodingVarint,
	ParameterTypeExpires:                 ParameterEncodingVarint,
	ParameterTypeLargestObject:           ParameterEncodingLocation,
	ParameterTypeFillTimeout:             ParameterEncodingVarint,
	ParameterTypeForward:                 ParameterEncodingUint8,
	ParameterTypeSubscriberPriority:      ParameterEncodingUint8,
	ParameterTypeSubscriptionFilter:      ParameterEncodingBytes,
	ParameterTypeGroupOrder:              ParameterEncodingUint8,
	ParameterTypeNewGroupRequest:         ParameterEncodingVarint,
	ParameterTypeTrackNamespacePrefix:    ParameterEncodingNamespace,
}

var ErrUnknownParameter = errors.New("unknown parameter type")

// Parameter is a Message Parameter. Only the value field matching the encoding
// of Type is used.
type Parameter struct {
	Type      uint64
	Uint8     uint8
	Varint    uint64
	Location  Location
	Bytes     []byte
	Namespace [][]byte
}

func (p *Parameter) Encoding() ParameterEncoding {
	return parameterEncodings[p.Type]
}

// append_v18 writes the value. An unknown type has no encoding and writes
// nothing.
func (p *Parameter) append_v18(buf []byte) []byte {
	switch p.Encoding() {
	case ParameterEncodingUint8:
		buf = append(buf, p.Uint8)
	case ParameterEncodingVarint:
		buf = varint.Append(buf, p.Varint)
	case ParameterEncodingLocation:
		buf = p.Location.append_v18(buf)
	case ParameterEncodingBytes:
		buf = varint.Append(buf, uint64(len(p.Bytes)))
		buf = append(buf, p.Bytes...)
	case ParameterEncodingNamespace:
		buf = varint.Append(buf, uint64(len(p.Namespace)))
		for _, v := range p.Namespace {
			buf = varint.Append(buf, uint64(len(v)))
			buf = append(buf, v...)
		}
	}
	return buf
}

func (p *Parameter) parse_v18(r messageReader) error {
	var err error
	switch p.Encoding() {
	case ParameterEncodingUint8:
		p.Uint8, err = r.ReadByte()
		return err
	case ParameterEncodingVarint:
		p.Varint, err = varint.Read(r)
		return err
	case ParameterEncodingLocation:
		return p.Location.parse_v18(r)
	case ParameterEncodingBytes:
		n, err := varint.Read(r)
		if err != nil {
			return err
		}
		p.Bytes, err = readBytes(r, n)
		return err
	case ParameterEncodingNamespace:
		n, err := varint.Read(r)
		if err != nil {
			return err
		}
		p.Namespace = make([][]byte, 0, min(n, 32))
		for range n {
			l, err := varint.Read(r)
			if err != nil {
				return err
			}
			v, err := readBytes(r, l)
			if err != nil {
				return err
			}
			p.Namespace = append(p.Namespace, v)
		}
		return nil
	}
	return fmt.Errorf("%w: 0x%x", ErrUnknownParameter, p.Type)
}

func compareParameters(a, b Parameter) int {
	if a.Type < b.Type {
		return -1
	}
	if a.Type > b.Type {
		return 1
	}
	return 0
}

// appendParameters_v18 writes a count prefixed parameter list, encoding each
// type as a delta from the previous one. The parameters are sorted by type
// first, because a delta encoding cannot express a decreasing sequence.
func appendParameters_v18(buf []byte, params []Parameter) []byte {
	if !slices.IsSortedFunc(params, compareParameters) {
		params = slices.Clone(params)
		slices.SortStableFunc(params, compareParameters)
	}
	buf = varint.Append(buf, uint64(len(params)))
	prev := uint64(0)
	for i := range params {
		buf = varint.Append(buf, params[i].Type-prev)
		buf = params[i].append_v18(buf)
		prev = params[i].Type
	}
	return buf
}

func parseParameters_v18(r messageReader) ([]Parameter, error) {
	count, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	params := make([]Parameter, 0, min(count, 32))
	prev := uint64(0)
	for range count {
		delta, err := varint.Read(r)
		if err != nil {
			return nil, err
		}
		p := Parameter{Type: prev + delta}
		if err := p.parse_v18(r); err != nil {
			return nil, err
		}
		prev = p.Type
		params = append(params, p)
	}
	return params, nil
}
