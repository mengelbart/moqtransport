package moqtransport

import (
	"errors"
	"log"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errUnknownParameter   = errors.New("unknown parameter")
	errDuplicateParameter = errors.New("duplicated parameter")
)

const (
	roleParameterKey uint64 = iota
	pathParameterKey
)

const (
	ingestionRole uint64 = iota + 1
	deliveryRole
	ingestionDeliveryRole
)

type parameter interface {
	append([]byte) []byte
	key() uint64
}

type parameters map[uint64]parameter

func parseParameter(r messageReader) (parameter, error) {
	key, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	log.Printf("reading param: %v", key)
	switch key {
	case roleParameterKey:
		return parseVarintParameter(r, key)
	case pathParameterKey:
		return parseStringParameter(r, key)
	}
	return nil, errUnknownParameter
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
		if _, ok := ps[p.key()]; ok {
			return nil, errDuplicateParameter
		}
		ps[p.key()] = p
	}
	return ps, nil
}
