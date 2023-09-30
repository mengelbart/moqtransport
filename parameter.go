package moqtransport

import (
	"errors"
	"io"

	"gitlab.lrz.de/cm/moqtransport/varint"
)

var (
	errUnknownParameter = errors.New("unknown parameter")
)

type parameterKey uint64

const (
	roleParameterKey parameterKey = iota
	pathParameterKey
)

const (
	ingestionRole roleParameter = iota + 1
	derliveryRole
	ingestionDeliveryRole
)

type Parameter interface {
	Key() parameterKey
	Append([]byte) []byte
	Len() uint64
}

func parseParameter(r MessageReader) (Parameter, int, error) {
	key, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, 0, err
	}
	switch parameterKey(key) {
	case roleParameterKey:
		rp, m, err := parseRoleParameter(r)
		if err != nil {
			return nil, 0, err
		}
		return rp, n + m, nil
	case pathParameterKey:
		pp, m, err := parsePathParameter(r)
		if err != nil {
			return nil, 0, err
		}
		return pp, n + m, nil
	}
	return nil, 0, errUnknownParameter
}

func parseRoleParameter(r MessageReader) (roleParameter, int, error) {
	l, err := varint.Read(r)
	if err != nil {
		return 0, 0, err
	}
	if l > 1 {
		return 0, 0, errInvalidMessageEncoding
	}
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	return roleParameter(b), 2, nil
}

func parsePathParameter(r MessageReader) (pathParameter, int, error) {
	l, n, err := varint.ReadWithLen(r)
	if err != nil {
		return "", 0, err
	}
	path := make([]byte, l)
	m, err := io.ReadFull(r, path)
	if err != nil {
		return "", 0, err
	}
	if l != uint64(m) {
		return "", 0, errInvalidMessageEncoding
	}
	return pathParameter(path), n + m, nil
}

// Don't confuse with role in message.go, this one is for sending and receiving
// role parameters, which are for identifying senders and receivers. The one in
// message.go is an internal variable to distinguish between client and server
// setup messages.
type roleParameter byte

func (r roleParameter) Key() parameterKey {
	return roleParameterKey
}

func (r roleParameter) Len() uint64 {
	return 1
}

func (p roleParameter) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(p.Key()))
	buf = varint.Append(buf, p.Len())
	return append(buf, byte(p))
}

type pathParameter string

func (p pathParameter) Key() parameterKey {
	return pathParameterKey
}

func (p pathParameter) Len() uint64 {
	return uint64(len(p))
}

func (p pathParameter) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(p.Key()))
	buf = varint.Append(buf, p.Len())
	return append(buf, p...)
}

type Parameters map[parameterKey]Parameter

func (p Parameters) Len() uint64 {
	l := uint64(0)
	for _, x := range p {
		keyLen := varint.Len(uint64(x.Key()))
		lenLen := varint.Len(x.Len())
		l = l + keyLen + lenLen + x.Len()
	}
	return l
}

func parseParameters(r MessageReader, length int) (Parameters, error) {
	ps := Parameters{}
	i := 0
	for i < length {
		p, n, err := parseParameter(r)
		if err != nil {
			return nil, err
		}
		i += n
		ps[p.Key()] = p
	}
	if i > length {
		return Parameters{}, errInvalidMessageEncoding
	}
	return ps, nil
}
