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

type parameter interface {
	key() parameterKey
	append([]byte) []byte
	length() uint64
}

func parseParameter(r messageReader) (parameter, int, error) {
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

func parseRoleParameter(r messageReader) (roleParameter, int, error) {
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

func parsePathParameter(r messageReader) (pathParameter, int, error) {
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

func (r roleParameter) key() parameterKey {
	return roleParameterKey
}

func (r roleParameter) length() uint64 {
	return 1
}

func (p roleParameter) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(p.key()))
	buf = varint.Append(buf, p.length())
	return append(buf, byte(p))
}

type pathParameter string

func (p pathParameter) key() parameterKey {
	return pathParameterKey
}

func (p pathParameter) length() uint64 {
	return uint64(len(p))
}

func (p pathParameter) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(p.key()))
	buf = varint.Append(buf, p.length())
	return append(buf, p...)
}

type parameters map[parameterKey]parameter

func (p parameters) length() uint64 {
	l := uint64(0)
	for _, x := range p {
		keyLen := varint.Len(uint64(x.key()))
		lenLen := varint.Len(x.length())
		l = l + keyLen + lenLen + x.length()
	}
	return l
}

func parseParameters(r messageReader, length int) (parameters, error) {
	ps := parameters{}
	i := 0
	for i < length {
		p, n, err := parseParameter(r)
		if err != nil {
			return nil, err
		}
		i += n
		ps[p.key()] = p
	}
	if i > length {
		return parameters{}, errInvalidMessageEncoding
	}
	return ps, nil
}
