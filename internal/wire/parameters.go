package wire

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mengelbart/moqtransport/varint"
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

// ParameterScope identifies the message a parameter list belongs to. For
// REQUEST_UPDATE and REQUEST_OK it names the request the message refers to.
type ParameterScope uint32

const (
	ScopeSubscribe ParameterScope = 1 << iota
	ScopeSubscribeOk
	ScopePublish
	ScopePublishOk
	ScopeFetch
	ScopeTrackStatus
	ScopeTrackStatusOk
	ScopePublishNamespace
	ScopeSubscribeNamespace
	ScopeSubscribeTracks
	ScopeRequestUpdateSubscribe
	ScopeRequestUpdateFetch
	ScopeRequestUpdateNamespace
	ScopeRequestUpdateOther
	ScopeRequestUpdateOk
)

const scopeRequestUpdateAny = ScopeRequestUpdateSubscribe | ScopeRequestUpdateFetch | ScopeRequestUpdateNamespace | ScopeRequestUpdateOther

const (
	DefaultSubscriberPriority uint8  = 128
	DefaultForward            uint8  = 1
	DefaultRendezvousTimeout  uint64 = 0
	DefaultExpires            uint64 = 0
)

const (
	GroupOrderAscending  uint8 = 0x1
	GroupOrderDescending uint8 = 0x2
)

type parameterDefinition struct {
	name     string
	encoding ParameterEncoding
	scopes   ParameterScope
	repeat   bool
	// min and max bound uint8 values, max is unbounded when zero.
	min, max uint8
}

var parameterDefinitions = map[uint64]parameterDefinition{
	ParameterTypeObjectDeliveryTimeout: {
		name:     "OBJECT_DELIVERY_TIMEOUT",
		encoding: ParameterEncodingVarint,
		scopes:   ScopePublishOk | ScopeSubscribe | ScopeRequestUpdateSubscribe,
	},
	ParameterTypeAuthorizationToken: {
		name:     "AUTHORIZATION_TOKEN",
		encoding: ParameterEncodingBytes,
		scopes: ScopePublish | ScopeSubscribe | scopeRequestUpdateAny | ScopeSubscribeNamespace |
			ScopeSubscribeTracks | ScopePublishNamespace | ScopeTrackStatus | ScopeFetch,
		repeat: true,
	},
	ParameterTypeRendezvousTimeout: {
		name:     "RENDEZVOUS_TIMEOUT",
		encoding: ParameterEncodingVarint,
		scopes:   ScopeSubscribe,
	},
	ParameterTypeSubgroupDeliveryTimeout: {
		name:     "SUBGROUP_DELIVERY_TIMEOUT",
		encoding: ParameterEncodingVarint,
		scopes:   ScopePublishOk | ScopeSubscribe | ScopeRequestUpdateSubscribe,
	},
	ParameterTypeExpires: {
		name:     "EXPIRES",
		encoding: ParameterEncodingVarint,
		scopes:   ScopeSubscribeOk | ScopePublish | ScopePublishOk | ScopeRequestUpdateOk,
	},
	ParameterTypeLargestObject: {
		name:     "LARGEST_OBJECT",
		encoding: ParameterEncodingLocation,
		scopes:   ScopeSubscribeOk | ScopePublish | ScopeRequestUpdateOk | ScopeTrackStatusOk,
	},
	ParameterTypeFillTimeout: {
		name:     "FILL_TIMEOUT",
		encoding: ParameterEncodingVarint,
		scopes:   ScopeFetch,
	},
	ParameterTypeForward: {
		name:     "FORWARD",
		encoding: ParameterEncodingUint8,
		scopes:   ScopeSubscribe | ScopeRequestUpdateSubscribe | ScopePublish | ScopePublishOk | ScopeSubscribeTracks,
		max:      1,
	},
	ParameterTypeSubscriberPriority: {
		name:     "SUBSCRIBER_PRIORITY",
		encoding: ParameterEncodingUint8,
		scopes:   ScopeSubscribe | ScopeFetch | ScopeRequestUpdateSubscribe | ScopeRequestUpdateFetch | ScopePublishOk,
	},
	ParameterTypeSubscriptionFilter: {
		name:     "SUBSCRIPTION_FILTER",
		encoding: ParameterEncodingBytes,
		scopes:   ScopeSubscribe | ScopePublishOk | ScopeRequestUpdateSubscribe,
	},
	ParameterTypeGroupOrder: {
		name:     "GROUP_ORDER",
		encoding: ParameterEncodingUint8,
		scopes:   ScopeSubscribe | ScopePublishOk | ScopeFetch,
		min:      GroupOrderAscending,
		max:      GroupOrderDescending,
	},
	ParameterTypeNewGroupRequest: {
		name:     "NEW_GROUP_REQUEST",
		encoding: ParameterEncodingVarint,
		scopes:   ScopePublishOk | ScopeSubscribe | ScopeRequestUpdateSubscribe,
	},
	ParameterTypeTrackNamespacePrefix: {
		name:     "TRACK_NAMESPACE_PREFIX",
		encoding: ParameterEncodingNamespace,
		scopes:   ScopeRequestUpdateNamespace,
	},
}

// Parameter validation errors, all close the session with PROTOCOL_VIOLATION.
var (
	ErrUnknownParameter   = errors.New("unknown parameter type")
	ErrParameterScope     = errors.New("parameter not allowed in message")
	ErrDuplicateParameter = errors.New("duplicate parameter")
	ErrParameterValue     = errors.New("invalid parameter value")
)

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
	return parameterDefinitions[p.Type].encoding
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

// ValidateParameters checks a parameter list against the definitions for
// scope. The returned error wraps one of the Err* sentinels.
func ValidateParameters(scope ParameterScope, params []Parameter) error {
	seen := make(map[uint64]struct{}, len(params))
	for i := range params {
		p := &params[i]
		def, ok := parameterDefinitions[p.Type]
		if !ok {
			return fmt.Errorf("%w: 0x%x", ErrUnknownParameter, p.Type)
		}
		if def.scopes&scope == 0 {
			return fmt.Errorf("%w: %s", ErrParameterScope, def.name)
		}
		if _, dup := seen[p.Type]; dup && !def.repeat {
			return fmt.Errorf("%w: %s", ErrDuplicateParameter, def.name)
		}
		seen[p.Type] = struct{}{}
		if def.encoding == ParameterEncodingUint8 && (p.Uint8 < def.min || (def.max > 0 && p.Uint8 > def.max)) {
			return fmt.Errorf("%w: %s %d", ErrParameterValue, def.name, p.Uint8)
		}
	}
	return nil
}

func findParameter(params []Parameter, t uint64) (*Parameter, bool) {
	for i := range params {
		if params[i].Type == t {
			return &params[i], true
		}
	}
	return nil, false
}

// Uint8ParameterValue returns the first parameter of type t, or def when absent.
func Uint8ParameterValue(params []Parameter, t uint64, def uint8) uint8 {
	if p, ok := findParameter(params, t); ok {
		return p.Uint8
	}
	return def
}

// VarintParameterValue returns the first parameter of type t, or def when absent.
func VarintParameterValue(params []Parameter, t uint64, def uint64) uint64 {
	if p, ok := findParameter(params, t); ok {
		return p.Varint
	}
	return def
}

// BytesParameterValue returns the first parameter of type t and whether it was present.
func BytesParameterValue(params []Parameter, t uint64) ([]byte, bool) {
	if p, ok := findParameter(params, t); ok {
		return p.Bytes, true
	}
	return nil, false
}

// AllBytesParameterValues returns the values of every parameter of type t in order.
func AllBytesParameterValues(params []Parameter, t uint64) [][]byte {
	var values [][]byte
	for i := range params {
		if params[i].Type == t {
			values = append(values, params[i].Bytes)
		}
	}
	return values
}

// LocationParameterValue returns the first parameter of type t and whether it was present.
func LocationParameterValue(params []Parameter, t uint64) (Location, bool) {
	if p, ok := findParameter(params, t); ok {
		return p.Location, true
	}
	return Location{}, false
}
