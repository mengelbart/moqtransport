package moqtransport

import (
	"encoding/binary"
	"errors"
	"log"
	"math"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var errControlMessageTooLarge = errors.New("control message too large")

func compileMessage(msg wire.ControlMessage) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	buf = quicvarint.Append(buf, uint64(msg.Type()))
	tl := len(buf)
	buf = append(buf, 0x00, 0x00) // length placeholder
	buf = msg.Append(buf)
	length := len(buf[tl+2:])
	if length > math.MaxUint16 {
		return nil, errControlMessageTooLarge
	}
	binary.BigEndian.PutUint16(buf[tl:tl+2], uint16(length))
	log.Printf("tl=%v, length=%v, buf=%v", tl, length, buf)
	return buf, nil
}

func validatePathParameter(setupParameters wire.Parameters, protocolIsQUIC bool) (string, error) {
	pathParam, ok := setupParameters[wire.PathParameterKey]
	if !ok {
		if !protocolIsQUIC {
			return "", nil
		}
		return "", errMissingPathParameter
	}
	if !protocolIsQUIC {
		return "", errUnexpectedPathParameter
	}
	pathParamValue, ok := pathParam.(wire.StringParameter)
	if !ok {
		return "", errInvalidPathParameterType
	}
	return pathParamValue.Value, nil
}

func validateMaxRequestIDParameter(setupParameters wire.Parameters) (uint64, error) {
	maxRequestIDParam, ok := setupParameters[wire.MaxRequestIDParameterKey]
	if !ok {
		return 0, nil
	}
	maxRequestIDParamValue, ok := maxRequestIDParam.(wire.VarintParameter)
	if !ok {
		return 0, errInvalidMaxRequestIDParameterType
	}
	return maxRequestIDParamValue.Value, nil
}

func validateAuthParameter(subscribeParameters wire.Parameters) (string, error) {
	authParam, ok := subscribeParameters[wire.AuthorizationParameterKey]
	if !ok {
		return "", nil
	}
	authParamValue, ok := authParam.(wire.StringParameter)
	if !ok {
		return "", errInvalidAuthParameterType
	}
	return authParamValue.Value, nil
}
