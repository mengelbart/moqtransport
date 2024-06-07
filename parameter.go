package moqtransport

import (
	"errors"
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errDuplicateParameter = errors.New("duplicated parameter")
)

const (
	roleParameterKey uint64 = iota
	pathParameterKey
	authorizationParameterKey
)

type parameter interface {
	append([]byte) []byte
	key() uint64
	String() string
}

type parameters map[uint64]parameter

func (p parameters) String() string {
	res := ""
	for k, v := range p {
		res += fmt.Sprintf("%v - {%v}\n", k, v.String())
	}
	return res
}

func parseParameter(r messageReader) (parameter, error) {
	key, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	switch key {
	case roleParameterKey:
		return parseVarintParameter(r, key)
	case pathParameterKey, authorizationParameterKey:
		return parseStringParameter(r, key)
	}
	length, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	_, err = r.Read(buf)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func parseParameters(r messageReader) (parameters, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	ps := parameters{}
	numParameters, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < numParameters; i++ {
		p, err := parseParameter(r)
		if err != nil {
			return nil, err
		}
		if p == nil {
			continue
		}
		if _, ok := ps[p.key()]; ok {
			return nil, errDuplicateParameter
		}
		ps[p.key()] = p
	}
	return ps, nil
}
