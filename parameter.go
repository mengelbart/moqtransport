package moqtransport

import (
	"errors"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errUnknownParameter   = errors.New("unknown parameter")
	errDuplicateParameter = errors.New("duplicated parameter")
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

func parseSetupParameter(r messageReader) (parameter, error) {
	return nil, nil
}

func parseParameter(r messageReader) (parameter, error) {
	key, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	switch parameterKey(key) {
	case roleParameterKey:
		rp, err := parseRoleParameter(r)
		if err != nil {
			return nil, err
		}
		return rp, nil
	case pathParameterKey:
		pp, err := parsePathParameter(r)
		if err != nil {
			return nil, err
		}
		return pp, nil
	}
	return nil, errUnknownParameter
}

func parseRoleParameter(r messageReader) (roleParameter, error) {
	l, err := quicvarint.Read(r)
	if err != nil {
		return 0, err
	}
	if l > 1 {
		return 0, errInvalidMessageEncoding
	}
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	return roleParameter(b), nil
}

func parsePathParameter(r messageReader) (pathParameter, error) {
	l, err := quicvarint.Read(r)
	if err != nil {
		return "", err
	}
	path := make([]byte, l)
	m, err := io.ReadFull(r, path)
	if err != nil {
		return "", err
	}
	if l != uint64(m) {
		return "", errInvalidMessageEncoding
	}
	return pathParameter(path), nil
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
	buf = quicvarint.Append(buf, uint64(p.key()))
	buf = quicvarint.Append(buf, p.length())
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
	buf = quicvarint.Append(buf, uint64(p.key()))
	buf = quicvarint.Append(buf, p.length())
	return append(buf, p...)
}

type parameters map[parameterKey]parameter

func parseParameters(r messageReader) (parameters, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	ps := parameters{}
	numParameters, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	set := map[parameterKey]struct{}{}
	for i := 0; i < int(numParameters); i++ {
		p, err := parseParameter(r)
		if err != nil {
			return nil, err
		}
		if _, ok := set[p.key()]; ok {
			return nil, errDuplicateParameter
		}
		set[p.key()] = struct{}{}
		ps[p.key()] = p
	}
	return ps, nil
}
